# Namespaces

Inspired by [Plan 9's namespaces](http://doc.cat-v.org/plan_9/4th_edition/papers/names) (which were later copied to Linux), Firefly uses namespaces to control access to shared resources, such as the network, the filesystem, and other processes. A process can create a new namespace of each type (subject to the its resource limits). These namespaces can then then be attached to newly-created processes in place of the parent process's namespaces. For example, if process A starts process B without specifying a network namespace, B will inherit A's namespace and the two will share a single network stack. If A then creates a new network namespace and uses it when starting process C, C will have an independent network stack, separate from the one used by A and B. A could then start process D, specifying the new namespace to allow C and D to share their network stack.

Each namespace is stored in a separate kernel structure, ensuring that two namespaces of the same type cannot see or interfere with one another.

## Types

The list of namespace types is as follows:

- Process (process ID)
- Filesystem
- Network
