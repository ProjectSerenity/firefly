package cc

var hdr_stdarg_h = `
typedef struct va_list *va_list;

void va_start(va_list, void*);
void va_end(va_list);
`
