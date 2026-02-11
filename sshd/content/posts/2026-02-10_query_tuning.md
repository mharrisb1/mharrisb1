---
title: "Postgres Query Tuning"
date: 2026-02-10
tags: [databases, postgres, sql]
summary: Brief overview to basic query tuning in Postgres
draft: false
---

# Basic Query Tuning in Postgres to Find Missing Indices

We were having an issue in the application this week where the automation engine I wrote[^1] had some poor performing read queries. The issue was the query patterns being used for runs, attempts, and logs.

In our platform, the automation engine supports thousands of user-created jobs per day. Each one of those jobs have a run and each run has N attempts which is configurable. Within an attempt there are also logs. There are several thousand to several tens of thousands of records added each day and growing and we finally hit a point where queries were running pretty slow.

Here is a brief overview of the data model:

```
[Trigger]-|--|-[Job]-|--o-[Destination]
                 |
                ---
                 |
                /|\
               [Run]
                 |
                ---
                 |
                /|\
             [Attempt]
                 |
                ---
                 |
                /|\
               [Log]
```

The route for listing attempts for a job run is `/v4/automations/jobs/{job_id}/runs/{run_id}/attempts` and the handler included a query that needed to do some joins. To load the dashboard overview for a team's automations (which was inspired by Airflow's DAG overview dashboard), we fetch the latest N runs for each job and then all attempts for those runs so we can show a user what has recently failed, succeeded, and expired or what is currently in progress right now. This can end up requiring many, many list attempt calls[^2].

These queries aren't complicated, just simple select statements, some joins, some filters, a limit, an offset, and an order by so I knew there is probably an obvious issue. After analyzing the query, I was able to identify the one missing index which would have prevented the performance issue and patch.

To showcase that analysis, I don't want to use our real production data and the development environment won't produce correct query plans[^3] so let's use this fake, but still illustrative example:

```sql
SELECT
 o.id,
 o.created_at,
 o.total,
 p.name,
 li.quantity
FROM orders o
 JOIN line_items li ON li.order_id = o.id
 JOIN products p ON p.id = li.product_id
WHERE o.customer_id = 1047
ORDER BY o.created_at DESC
LIMIT 20;
```

This is a pretty common data model for CRMs where you have an `order` which contains `line_items` which reference `products`.

To analyze how this query would perform you would just add `EXPLAIN (ANALYZE, BUFFERS)` to the top of the query.

> [!CAUTION]
> when using `ANALYZE`, the query is actually ran so if you have a query with mutations you must wrap in a rollback

So now the query is:

```sql
EXPLAIN (ANALYZE, BUFFERS)
SELECT
 o.id,
 o.created_at,
 o.total,
 p.name,
 li.quantity
FROM orders o
 JOIN line_items li ON li.order_id = o.id
 JOIN products p ON p.id = li.product_id
WHERE o.customer_id = 1047
ORDER BY o.created_at DESC
LIMIT 20;
```

I don't want to go deep on all the options for `EXPLAIN` or what the options I used do but in short, `EXPLAIN` by itself will not run the actual query and will give you an estimated query plan. By using the `ANALYZE` option, the query _is_ ran so you get a real query plan that was actually used. See caution note above on this to avoid footguns. I also added `BUFFER` mainly because I always do when I analyze though this issue didn't really benefit from that. That option will also include I/O accounting.

See [documentation](https://www.postgresql.org/docs/current/sql-explain.html).

From running the query above with the explain command, we get this:

```
Limit  (cost=45032.10..45032.15 rows=20 width=52)
       (actual time=812.331..812.340 rows=20 loops=1)
  Buffers: shared hit=1204 read=38710
  ->  Sort  (cost=45032.10..45033.22 rows=448 width=52)
            (actual time=812.328..812.333 rows=20 loops=1)
        Sort Key: o.created_at DESC
        Sort Method: top-N heapsort  Memory: 27kB
        Buffers: shared hit=1204 read=38710
        ->  Nested Loop  (cost=1000.58..45021.33 rows=448 width=52)
                         (actual time=7.112..811.942 rows=83 loops=1)
              Buffers: shared hit=1204 read=38710
              ->  Nested Loop  (cost=1000.14..44883.12 rows=448 width=44)
                               (actual time=7.043..810.534 rows=83 loops=1)
                    Buffers: shared hit=1121 read=38710
                    ->  Gather Merge  (cost=1000.00..44398.90 rows=24 width=20)
                                      (actual time=6.801..806.498 rows=24 loops=1)
                          Workers Planned: 2
                          Workers Launched: 2
                          Buffers: shared hit=862 read=38544
                          ->  Parallel Seq Scan on orders o
                                (cost=0.00..43396.50 rows=10 width=20)
                                (actual time=3.912..800.211 rows=8 loops=3)
                                Filter: (customer_id = 1047)
                                Rows Removed by Filter: 666658
                                Buffers: shared hit=862 read=38544
                    ->  Seq Scan on line_items li
                          (cost=0.00..20.12 rows=19 width=24)
                          (actual time=0.152..0.163 rows=3 loops=24)
                          Filter: (order_id = o.id)
                          Rows Removed by Filter: 847
                          Buffers: shared hit=259 read=166
              ->  Index Scan using products_pkey on products p
                    (cost=0.43..0.31 rows=1 width=18)
                    (actual time=0.014..0.015 rows=1 loops=83)
                    Index Cond: (id = li.product_id)
                    Buffers: shared hit=83
```

If you've never seen a query plan before then this will look like a lot of stuff being thrown at you. Don't worry, it kind of gets easier.

The two main parts that we care about for initial tuning are:

1. `Parallel Seq Scan on orders o`
2. `Seq Scan on line_items li`

Generally, when I have a slow query and I see one or more sequential scans then I assume that is the issue. If we look at the surrounding metadata of those scans, you can see for orders the actual time runs from `3.912..800.211` so virtually 800ms of this query that in total took ~812ms was just scanning orders. We see the filter, `Filter: (customer_id = 1047)` is being used for all rows in the scan so it's checking every single row for a match. Even though there were 3 workers (one master and 2 launched) each node still had to scan 666k+ rows to look for a match.

It cost less total time to scan line items but we still had 1 node scanning 800+ rows looking for an exact match to the filter `Filter: (order_id = o.id)`.

To fix the performance here we can add 2 indexes:

```sql
-- Fix problem 1: find a customer's orders without scanning everything
CREATE INDEX CONCURRENTLY idx_orders_customer_created
  ON orders (customer_id, created_at DESC);

-- Fix problem 2: look up line items by order without scanning the table
CREATE INDEX CONCURRENTLY idx_line_items_order_id
  ON line_items (order_id);
```

Now when we run the same query again with explain we get:

```
Limit  (cost=2.14..18.92 rows=20 width=52)
       (actual time=0.087..0.214 rows=20 loops=1)
  Buffers: shared hit=91
  ->  Nested Loop  (cost=2.14..378.55 rows=448 width=52)
                   (actual time=0.085..0.208 rows=20 loops=1)
        Buffers: shared hit=91
        ->  Nested Loop  (cost=1.71..334.20 rows=448 width=44)
                         (actual time=0.071..0.161 rows=20 loops=1)
              Buffers: shared hit=8
              ->  Index Scan using idx_orders_customer_created on orders o
                    (cost=0.43..8.71 rows=24 width=20)
                    (actual time=0.031..0.042 rows=20 loops=1)
                    Index Cond: (customer_id = 1047)
                    Buffers: shared hit=4
              ->  Index Scan using idx_line_items_order_id on line_items li
                    (cost=1.28..13.52 rows=19 width=24)
                    (actual time=0.004..0.005 rows=3 loops=20)
                    Index Cond: (order_id = o.id)
                    Buffers: shared hit=4
        ->  Index Scan using products_pkey on products p
              (cost=0.43..0.31 rows=1 width=18)
              (actual time=0.002..0.002 rows=1 loops=20)
              Index Cond: (id = li.product_id)
              Buffers: shared hit=83
```

Immediately, we can see that we went from over 800ms to 0.2ms which is a 99.98% improvement. You can als see that we now have zero sequential scans ant orders and line items are using index scans.

This type of performance improvement is not uncommon. Simple identifying part of a query that are slow and then adding indexes or other tuning methods can get you big wins with very little effort.

## When shouldn't you tune?

There are many reasons why not tuning a slow query is the right call, but for me the main tradeoff is around query patterns. If a table or collection of tables in a query are high write, low read then you don't want to add a bunch of indexes that will speed up the few reads at the expense of all the writes. This is true for the counter where there is high reads, low rights. Tuning is context-dependent so it may seem awesome to go around finding missing indexes but just be aware there are costs to this.

---

[^1]: I'll try to get a post out on that sometime this week

[^2]: We are investigating alternatives like providing a route with full aggregate view or moving to GraphQL

[^3]: If there is not much data in the database than the query planner and optimizer will just use sequential scans instead of index scans which skews the analysis
