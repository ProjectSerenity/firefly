# 03-multi-package

This is an evolution of the previous examples, where some of the data
and functionality exists in a standalone package. This means that the
first (non-main) package is compiled first and the second (main)
package imports it, is compiled separately, and the two are linked
together.

Normally, different Ruse packages would be in different folders. This
example is in one folder to keep things simple. The Ruse compiler does
not enforce rules about to which folder(s) the files in a Ruse package
belong. As in this case, multiple packages can exist in one folder.
Similarly, a single package could be made from files in multiple folders,
but that is not recommended.
