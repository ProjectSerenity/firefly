# Plan interface description language

Plan is a minimal Lisp-like language used to define the system call ABI of an operating system.
With an OS's ABI defined in a Plan document, compatible code can be generated in other programming
languages. This ensures that user and kernel code do not disagree on the structure or layout of
the ABI.

Sample:

```lisp
(structure
	(name file info)
	(docs
		"Information about a file in the filesystem")
	(field
		(name name pointer)
		(docs
			"A pointer to the name of the file (UTF-8 encoded).")
		(type *constant byte))
	(field
		(name name length)
		(docs
			"The length in bytes of the file name.")
		(type uint64)))


(enumeration
	(name error)
	(docs
		"A common error that has been encountered "
		"while responding to a system call.")
	(type uint64)
	(value
		(name no error)
		(docs
			"The system call was successful and no error occurred."))
	(value
		(name bad syscall)
		(docs
			"The system call specified does not exist, or has not "
			"been implemented."))
	(value
		(name illegal parameter)
		(docs
			"An invalid or malformed parameter was provided to "
			"the system calls.")))


(syscall
	(name print message)
	(docs
		"Prints a message to the process's standard output.")
	(arg1
		(name pointer)
		(docs
			"The pointer to readable memory where the message resides. "
			"No restrictions are placed on the contents pointed to by "
			"`pointer`. For example, the contents do not need to be UTF-8 "
			"encoded and NUL bytes have no special effects.")
		(type *constant byte))
	(arg2
		(name length)
		(docs
			"The number of bytes that will be read and printed from `pointer`.")
		(type uint64))
	(result1
		(name written)
		(docs
			"The number of bytes that were read and printed from `pointer`.")
		(type uint64))
	(result2
		(name error)
		(docs
			"Any error encountered while printing the message.")
		(type error)))
```
