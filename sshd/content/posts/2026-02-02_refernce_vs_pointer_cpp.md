---
title: "References vs. Pointers in C++"
date: 2026-02-02
tags: [c++]
summary: "Slowly getting up to speed on C++ nuances"
draft: false
---

# References (`&`) vs Pointers (`*`)

From my research today, it seems the main difference comes down to two things:

1. Can it be nullable?
2. Is it borrowed or owned?

References _cannot_ be null. Having a reference in a function signature is a contract stating "I require a real thing".

Here's an example:

```cpp
void foo(vector<int>& nums);  // must refer to a real vector
void foo(vector<int>* nums);  // could be nullptr
```

With pointers, since there is no contract, you must defend every access (like in C).

References also cannot be reseated so you do not need to worry as the caller that the callee has nulled out a pointer.

```cpp
void foo(vector<int>* nums) {
    nums = nullptr;           // allowed
    nums = someOtherVector;   // allowed
}
```

This also leads into the intent around "borrowed, not owned". Most of the time, function arguments are meant to be borrowed. Using a reference strengthens that intention, though this is definitely a case where something like Rust has an even more explicit contract.
