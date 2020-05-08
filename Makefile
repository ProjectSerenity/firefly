.PHONY: all
all:
	$(MAKE) -C Pure64
	$(MAKE) -C kcheck
	$(MAKE) -C imgbuild
	$(MAKE) -C kernel
