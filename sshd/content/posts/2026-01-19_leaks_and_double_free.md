---
title: "Causing a double-free by fixing a leak"
date: 2026-01-19
tags: [c, gcc, gdb, asan]
summary: "Shooting myself in the foot by introducing a double free bug by trying to fix a simple memory leak"
draft: false
---

While working on my HashSet implementation in C, I accidentally had a memory leak:

```c
typedef struct llist_t {
    int             val;
    struct llist_t *next;
} llist_t;

llist_t *llist_new();
void     llist_free();

typedef struct {
    size_t    buckets;
    llist_t **chains;
} hashset_t;

hashset_t *hashset_new(size_t buckets);
hashset_t *hashset_default();

void hashset_free(hashset_t *set) {
    for (int i = 0; i < set->buckets; i++) {
        if (set->chains[i] != NULL) {
            llist_free(set->chains[i]);
        }
    }
    // HERE: did not free calloc'd chains
    free(set);
}
```

This was caught with [ASan](https://github.com/google/sanitizers/wiki/addresssanitizer):

```sh
gcc -fsanitize=address -g hashset.c -o hashset.o
./hashset.o

=================================================================
==2980933==ERROR: LeakSanitizer: detected memory leaks

Direct leak of 128 byte(s) in 1 object(s) allocated from:
    #0 0x7fa0cfcb83b7 in __interceptor_calloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:77
    #1 0x5624944694c9 in hashset_new /home/workstation/scratch/hashset.c:50
    #2 0x5624944695c3 in hashset_default /home/workstation/scratch/hashset.c:60
    #3 0x562494469786 in main /home/workstation/scratch/hashset.c:87
    #4 0x7fa0cfa45249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58

SUMMARY: AddressSanitizer: 128 byte(s) leaked in 1 allocation(s).
```

Going to line 50 in the scratch file showed the following so the remedy was easy, I had just forgotten to free the allocated array of pointers for the chains.

```c
    set->chains = calloc(set->buckets, sizeof(llist_t));
```

But, unfortunately I was sloppy and did this instead:

```c
void hashset_free(hashset_t *set) {
    for (int i = 0; i < set->buckets; i++) {
        if (set->chains[i] != NULL) {
            llist_free(set->chains[i]);
        }
        free(set->chains);
    }
    free(set);
}
```

Spot the mistake?

Running the program we get this:

```sh
./hashset.o
=================================================================
==2991931==ERROR: AddressSanitizer: heap-use-after-free on address 0x60c000000048 at pc 0x55b8d6fb1638 bp 0x7ffef522bb70 sp 0x7ffef522bb68
READ of size 8 at 0x60c000000048 thread T0
    #0 0x55b8d6fb1637 in hashset_free /home/workstation/scratch/hashset.c:65
    #1 0x55b8d6fb17cb in main /home/workstation/scratch/hashset.c:88
    #2 0x7f7039a45249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7f7039a45304 in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x55b8d6fb1150 in _start (/home/workstation/scratch/hashset.o+0x1150)

0x60c000000048 is located 8 bytes inside of 128-byte region [0x60c000000040,0x60c0000000c0)
freed by thread T0 here:
    #0 0x7f7039cb76a8 in __interceptor_free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52
    #1 0x55b8d6fb16b4 in hashset_free /home/workstation/scratch/hashset.c:68
    #2 0x55b8d6fb17cb in main /home/workstation/scratch/hashset.c:88
    #3 0x7f7039a45249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58

previously allocated by thread T0 here:
    #0 0x7f7039cb83b7 in __interceptor_calloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:77
    #1 0x55b8d6fb14c9 in hashset_new /home/workstation/scratch/hashset.c:50
    #2 0x55b8d6fb15c3 in hashset_default /home/workstation/scratch/hashset.c:60
    #3 0x55b8d6fb17bb in main /home/workstation/scratch/hashset.c:87
    #4 0x7f7039a45249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58

SUMMARY: AddressSanitizer: heap-use-after-free /home/workstation/scratch/hashset.c:65 in hashset_free
Shadow bytes around the buggy address:
  0x0c187fff7fb0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fc0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fd0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fe0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7ff0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
=>0x0c187fff8000: fa fa fa fa fa fa fa fa fd[fd]fd fd fd fd fd fd
  0x0c187fff8010: fd fd fd fd fd fd fd fd fa fa fa fa fa fa fa fa
  0x0c187fff8020: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8030: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8040: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8050: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
Shadow byte legend (one shadow byte represents 8 application bytes):
  Addressable:           00
  Partially addressable: 01 02 03 04 05 06 07
  Heap left redzone:       fa
  Freed heap region:       fd
  Stack left redzone:      f1
  Stack mid redzone:       f2
  Stack right redzone:     f3
  Stack after return:      f5
  Stack use after scope:   f8
  Global redzone:          f9
  Global init order:       f6
  Poisoned by user:        f7
  Container overflow:      fc
  Array cookie:            ac
  Intra object redzone:    bb
  ASan internal:           fe
  Left alloca redzone:     ca
  Right alloca redzone:    cb
==2991931==ABORTING
```

`AddressSanitizer` is very helpful and explicitly tells us that we just tried to do `heap-use-after-free` on line 65 which is:

```c
    if (set->chains[i] != NULL) {
```

Because I had still not spotted the obvious mistake, I wasn't sure if the double-free'd memory was the chains array or the actual linked list struct at index `i` so I ran GDB:

```sh
GNU gdb (Debian 13.1-3) 13.1
Copyright (C) 2023 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
Type "show copying" and "show warranty" for details.
This GDB was configured as "x86_64-linux-gnu".
Type "show configuration" for configuration details.
For bug reporting instructions, please see:
<https://www.gnu.org/software/gdb/bugs/>.
Find the GDB manual and other documentation resources online at:
    <http://www.gnu.org/software/gdb/documentation/>.

For help, type "help".
Type "apropos word" to search for commands related to "word"...
Reading symbols from ./hashset.o...
(gdb) b hashset_free
Breakpoint 1 at 0x15d2: file hashset.c, line 64.
(gdb) r
Starting program: /home/workstation/scratch/hashset.o
[Thread debugging using libthread_db enabled]
Using host libthread_db library "/lib/x86_64-linux-gnu/libthread_db.so.1".

Breakpoint 1, hashset_free (set=0x602000000090) at hashset.c:64
64        for (int i = 0; i < set->buckets; i++) {
(gdb) watch i
Hardware watchpoint 2: i
(gdb) n

Hardware watchpoint 2: i

Old value = 21845
New value = 0
hashset_free (set=0x602000000090) at hashset.c:64
64        for (int i = 0; i < set->buckets; i++) {
(gdb) n
65          if (set->chains[i] != NULL) {
(gdb) n
68          free(set->chains);
(gdb) n
64        for (int i = 0; i < set->buckets; i++) {
(gdb) n

Hardware watchpoint 2: i

Old value = 0
New value = 1
0x00005555555556b9 in hashset_free (set=0x602000000090) at hashset.c:64
64        for (int i = 0; i < set->buckets; i++) {
(gdb) n
65          if (set->chains[i] != NULL) {
(gdb) n
=================================================================
==2997308==ERROR: AddressSanitizer: heap-use-after-free on address 0x60c000000048 at pc 0x555555555638 bp 0x7fffffffde40 sp 0x7fffffffde38
READ of size 8 at 0x60c000000048 thread T0
    #0 0x555555555637 in hashset_free /home/workstation/scratch/hashset.c:65
    #1 0x5555555557cb in main /home/workstation/scratch/hashset.c:88
    #2 0x7ffff7645249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7ffff7645304 in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x555555555150 in _start (/home/workstation/scratch/hashset.o+0x1150)

0x60c000000048 is located 8 bytes inside of 128-byte region [0x60c000000040,0x60c0000000c0)
freed by thread T0 here:
    #0 0x7ffff78b76a8 in __interceptor_free ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:52
    #1 0x5555555556b4 in hashset_free /home/workstation/scratch/hashset.c:68
    #2 0x5555555557cb in main /home/workstation/scratch/hashset.c:88
    #3 0x7ffff7645249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58

previously allocated by thread T0 here:
    #0 0x7ffff78b83b7 in __interceptor_calloc ../../../../src/libsanitizer/asan/asan_malloc_linux.cpp:77
    #1 0x5555555554c9 in hashset_new /home/workstation/scratch/hashset.c:50
    #2 0x5555555555c3 in hashset_default /home/workstation/scratch/hashset.c:60
    #3 0x5555555557bb in main /home/workstation/scratch/hashset.c:87
    #4 0x7ffff7645249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58

SUMMARY: AddressSanitizer: heap-use-after-free /home/workstation/scratch/hashset.c:65 in hashset_free
Shadow bytes around the buggy address:
  0x0c187fff7fb0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fc0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fd0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7fe0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
  0x0c187fff7ff0: 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 00
=>0x0c187fff8000: fa fa fa fa fa fa fa fa fd[fd]fd fd fd fd fd fd
  0x0c187fff8010: fd fd fd fd fd fd fd fd fa fa fa fa fa fa fa fa
  0x0c187fff8020: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8030: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8040: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
  0x0c187fff8050: fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa fa
Shadow byte legend (one shadow byte represents 8 application bytes):
  Addressable:           00
  Partially addressable: 01 02 03 04 05 06 07
  Heap left redzone:       fa
  Freed heap region:       fd
  Stack left redzone:      f1
  Stack mid redzone:       f2
  Stack right redzone:     f3
  Stack after return:      f5
  Stack use after scope:   f8
  Global redzone:          f9
  Global init order:       f6
  Poisoned by user:        f7
  Container overflow:      fc
  Array cookie:            ac
  Intra object redzone:    bb
  ASan internal:           fe
  Left alloca redzone:     ca
  Right alloca redzone:    cb
==2997308==ABORTING
[Inferior 1 (process 2997308) exited with code 01]
(gdb)
```

This helped because I was able to see the error occurred at index 1. Inspecting that, I was able to see that the linked list node at index 1 is NULL and therefore could not have been double-freed:

```sh
(gdb) p set->chains[1]
$3 = (llist_t *) 0x0
```

That left only the chains array itself. And stepping through the debugger again it became clear to me what the problem was when I saw that `free(set->chains)` happen twice with the second causing the double-free error. So I issued a fix with:

```diff
void hashset_free(hashset_t *set) {
    for (int i = 0; i < set->buckets; i++) {
        if (set->chains[i] != NULL) {
            llist_free(set->chains[i]);
        }
-       free(set->chains);
    }
+   free(set->chains);
    free(set);
}
```

# Other takeaways

Something I need to be better about is explicitly nullifying pointers after free to make sure that I don't run into a double-free. However, even if I would have done this instead:

```diff
void hashset_free(hashset_t *set) {
    for (int i = 0; i < set->buckets; i++) {
        if (set->chains[i] != NULL) {
            llist_free(set->chains[i]);
        }
        free(set->chains);
+       set->chains = NULL;
    }
    free(set);
}
```

I would have ran into a segfault with a different error message:

```sh
AddressSanitizer:DEADLYSIGNAL
=================================================================
==3568434==ERROR: AddressSanitizer: SEGV on unknown address 0x000000000008 (pc 0x55adaa496638 bp 0x7ffc5b8180d0 sp 0x7ffc5b8180b0 T0)
==3568434==The signal is caused by a READ memory access.
==3568434==Hint: address points to the zero page.
    #0 0x55adaa496638 in hashset_free /home/workstation/scratch/hashset.c:65
    #1 0x55adaa4967fc in main /home/workstation/scratch/hashset.c:88
    #2 0x7f1997a45249 in __libc_start_call_main ../sysdeps/nptl/libc_start_call_main.h:58
    #3 0x7f1997a45304 in __libc_start_main_impl ../csu/libc-start.c:360
    #4 0x55adaa496150 in _start (/home/workstation/scratch/hashset.o+0x1150)

AddressSanitizer can not provide additional info.
SUMMARY: AddressSanitizer: SEGV /home/workstation/scratch/hashset.c:65 in hashset_free
==3568434==ABORTING
```

This would have been due to trying to access an invalid address, `0x000000000008` which is 8 bytes/64 bits offset from `NULL` pointer which is exactly what you'd get when trying to `set->chains[1]` if `set->chains` is `NULL`.
