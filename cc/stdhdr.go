package cc

var hdr_stdarg_h = `
typedef struct va_list *va_list;
`

var hdr_extra_go_h = `
extern Node *N;
extern Sym *S;
extern Type *T;
extern Label *L;
//extern Case *C;
extern Prog *P;

enum
{
	BITS = 5,
	NVAR = BITS*4*8,
};
`

var hdr_sys_stat_h = `
struct stat {
	int st_mode;
};

int lstat(char*, struct stat*);
int S_ISREG(int);
`
