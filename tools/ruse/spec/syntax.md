# Ruse syntax

The Ruse syntax is radically simple, allowing a consistent [grammar](./grammar.md) to be built on top.

## Notation

The syntax is specified using a [variant](https://en.wikipedia.org/wiki/Wirth_syntax_notation) of Extended Backus-Naur Form (EBNF):

```
Syntax      = { Production } .
Production  = production_name "=" [ Expression ] "." .
Expression  = Term { "|" Term } .
Term        = Factor { Factor } .
Factor      = production_name | token [ "…" token ] | Group | Option | Repetition .
Group       = "(" Expression ")" .
Option      = "[" Expression "]" .
Repetition  = "{" Expression "}" .
```

Productions are expressions constructed from terms and the following operators, in increasing precedence:

```
|   alternation
()  grouping
[]  option (0 or 1 times)
{}  repetition (0 to n times)
```

Lowercase production names are used to identify lexical (terminal) tokens. Non-terminals are in CamelCase. Lexical tokens are enclosed in double quotes `""` or back quotes `\`\``.

The form `a … b` represents the set of characters from `a` to `b` as alternatives. The horizontal ellipsis `…` is also used elsewhere in the spec to informally denote various enumerations or code snippets that are not further specified. The character `…` (as opposed to the three characters `...`) is not a token of the Ruse language.

## Source code representation

Source code is Unicode text encoded in [UTF-8](https://en.wikipedia.org/wiki/UTF-8). The text is not canonicalised, so a single accented code point is distinct from the same character constructed from combining an accent and a letter; those are treated as two code points.  For simplicity, this document will use the unqualified term _character_ to refer to a Unicode code point in the source text.

Each code point is distinct; for instance, uppercase and lowercase letters are different characters.

Implementation restriction: For compatibility with other tools, a compiler may disallow the NUL character (U+0000) in the source text.

Implementation restriction: For compatibility with other tools, a compiler may ignore a UTF-8-encoded byte order mark (U+FEFF) if it is the first Unicode code point in the source text. A byte order mark may be disallowed anywhere else in the source.

### Characters

The following terms are used to denote specific Unicode character categories:

```
newline        = /* the Unicode code point U+000A */ .
unicode_char   = /* an arbitrary Unicode code point except newline */ .
unicode_letter = /* a Unicode code point categorized as "Letter" */ .
unicode_digit  = /* a Unicode code point categorized as "Number, decimal digit" */ .
```

In [The Unicode Standard 8.0](https://www.unicode.org/versions/Unicode8.0.0/), Section 4.5 "General Category" defines a set of character categories. Ruse treats all characters in any of the Letter categories Lu, Ll, Lt, Lm, or Lo as Unicode letters, and those in the Number category Nd as Unicode digits.

### Letters and digits

The underscore character `_` (U+005F) is considered a lowercase letter.

```
letter        = unicode_letter | "_" .
decimal_digit = "0" … "9" .
binary_digit  = "0" | "1" .
hex_digit     = "0" … "9" | "A" … "F" | "a" … "f" .
```

## Syntax elements

There are seven syntax elements in Ruse, each of which is described below:

1. Comments
2. Literals
3. Identifiers
4. Lists
5. Qualified identifiers
6. Quoted identifiers
7. Quoted lists

Literals, identifiers, and qualified identifiers are collectively referred to as _atoms_. Sequential atoms must be separated by whitespace. Quoted identifiers and quoted lists are collectively referred to as _metadata_, in that they affect the behaviour of Ruse programs but are not, themselves, executable. Comments do not affect compiled Ruse programs and thus are not metadata.

### Comments

Comments serve as program documentation. _Line comments_ start with a semicolon (`;`) and stop at the end of the line.

A comment cannot start inside a [string literal](#String_literals), or inside a comment. A comment acts like a newline.

```
; A line comment on its own.

(+ 1 2) ; A line comment after an expression.

(append
	strings  ; A line comment within an expression.
	text)

```

### Literals

Literals are the literal representation of constant values. Each literal is either an integer literal or a string literal.

#### Integer literals

An integer literal is a sequence of digits representing an integer constant. An optional prefix sets a non-decimal base: `0b` for binary and `0x` for hexadecimal. A single `0` is considered a decimal zero. In hexadecimal literals, letters `a` to `f` and `A` to `F` represent values 10 to 15.

For readability, an underscore character `_` may appear after a base prefix or between successive digits; such underscores do not change the literal's value.

```
int_lit        = decimal_lit | binary_lit | hex_lit .
decimal_lit    = "0" | ( "1" … "9" ) [ [ "_" ] decimal_digits ] .
binary_lit     = "0b" [ "_" ] binary_digits .
hex_lit        = "0x" [ "_" ] hex_digits .

decimal_digits = decimal_digit { [ "_" ] decimal_digit } .
binary_digits  = binary_digit { [ "_" ] binary_digit } .
hex_digits     = hex_digit { [ "_" ] hex_digit } .
```

```
42
4_2
0xBadFace
0xBad_Face
0x_67_7a_2f_cc_40_c6
170141183460469231731687303715884105727
170_141183_460469_231731_687303_715884_105727

_42         ; an identifier, not an integer literal
42_         ; invalid: _ must separate successive digits
4__2        ; invalid: only one _ at a time
0_xBadFace  ; invalid: _ must separate successive digits
```

#### String literals

A string literal represents a string constant obtained from concatenating a sequence of characters. String literals are character sequences between double quotes, as in `"bar"`. Within the quotes, any character may appear except newline and unescaped double quote. The text between the quotes forms the value of the literal, while multi-character sequences beginning with a backslash encode values in various formats.

Several backslash escapes allow arbitrary values to be encoded as ASCII text. There are three ways to insert a numeric constant into the string content: `\x` followed by exactly two hexadecimal digits; `\u` followed by exactly four hexadecimal digits, and a `\U` followed by exactly eight hexadecimal digits. In each case the value of the literal is the value represented by the digits in the corresponding base.

Although these representations all result in an integer being inserted into the string content, they have different valid ranges. The escapes `\u` and `\U` represent Unicode code points so within them some values are illegal, in particular those above `0x10FFFF` and surrogate halves.

After a backslash, certain single-character escapes represent special values:

```
\a   U+0007 alert or bell
\b   U+0008 backspace
\f   U+000C form feed
\n   U+000A line feed or newline
\r   U+000D carriage return
\t   U+0009 horizontal tab
\v   U+000B vertical tab
\\   U+005C backslash
\"   U+0022 double quote
```

An unrecognized character following a backslash in a string literal is illegal.

```
string_lit       = `"` { unicode_value | byte_value } `"` .
unicode_value    = unicode_char | little_u_value | big_u_value | escaped_char .
byte_value       = hex_byte_value .
hex_byte_value   = `\` "x" hex_digit hex_digit .
little_u_value   = `\` "u" hex_digit hex_digit hex_digit hex_digit .
big_u_value      = `\` "U" hex_digit hex_digit hex_digit hex_digit
                           hex_digit hex_digit hex_digit hex_digit .
escaped_char     = `\` ( "a" | "b" | "f" | "n" | "r" | "t" | "v" | `\` | `"` ) .
```

```
"\n"
"\""                 ; same as `"`
"Hello, world!\n"
"日本語"
"\u65e5本\U00008a9e"
"\xff\u00FF"
"\uD800"             ; illegal: surrogate half
"\U00110000"         ; illegal: invalid Unicode code point
```

These examples all represent the same string:

```
"日本語"                                 ; UTF-8 input text
"\u65e5\u672c\u8a9e"                    ; the explicit Unicode code points
"\U000065e5\U0000672c\U00008a9e"        ; the explicit Unicode code points
"\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e"  ; the explicit UTF-8 bytes
```

If the source code represents a character as two code points, such as a combining form involving an accent and a letter, the result will appear as two code points if placed in a string literal.

### Identifiers

Identifiers name program entities such as variables and types. An identifier is a sequence of one or more letters and digits, or a single plus or minus. The first character in an identifier must be a letter or punctuation.

```
identifier_initial    = letter | "!" | "$" | "%" | "&" | "*" | "/" | ":" | "<" | "=" | ">" | "?" | "@" | "~" | "_" | "^" | "|" .
identifier_subsequent = letter | unicode_digit | "+" | "-" .
identifier            = "+" | "-" | identifier_initial { identifier_subsequent } .
```

```
a
+
_x9
ThisVariableIsExported
αβ
kebab-case-is-fine/good
```

## Lists

A list is an ordered sequence of one or more syntax elements separated by whitespace, surrounded by parentheses. A list must always contain at least one atom or list. The number of whitespace characters between list elements does not change the meaning of the list.

```
(one)                ; A list containing a single identifier 'one'.
( one
	)                ; Another representation for the same list.
(one 2)              ; A list containing an identifier and an integer literal.
(("three") 4)        ; A list containing a list and an integer literal. The inner list contains a string literal.


)                    ; illegal: missing opening parenthesis.
()                   ; illegal: no elements.
('foo)               ; illegal: no atom or list elements.
```

### Qualified identifiers

A qualified identifier consists of two identifiers joined by a period. This is used to indicate that the identifier on the right is selecting a component within the object referenced by the identifier on the left. There must be no whitespace between either identifier and the joining period.

```
a.b                  ; A qualified identifier that selects 'b' within the object referenced by 'a'.
foo.bar              ; Selecting 'bar' within 'foo'.

a .b                 ; illegal: whitespace between 'a' and the joining period.
```

### Quoted identifiers

A quoted identifier consists of a single quote (`'`), followed immediately by an identifier. These are typically used as labels. There must be no whitespace beteween the quote and the identifier.

```
'foo                 ; A quoted identifier.

' foo                ; illegal: whitespace between the quote and the identifier.
```

### Quoted lists

A quoted list consists of a single quote (`'`), followed immediately by a list. These are typically used as annotations, to apply metadata to other Ruse code. There must be no whitespace between the quote and the identifier.

```
'(arch x86-64)       ; A quoted list containing two identifiers.

' (arch)             ; illegal: whitespace between the quote and the list.
```
