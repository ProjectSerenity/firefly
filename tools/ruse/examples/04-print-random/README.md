# 04-print-rnadom

This is a more complex package that uses a few different features:

- We use the X86 `rdrand` instruction to get a random 8-bit integer.
- We include a function that prints an 8-bit number as a hex string.
- We include an `if` expression to check whether the integer is even.
- We use a more complex test script to check the output.
