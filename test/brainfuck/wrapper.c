#include <stdio.h>
#include <stdlib.h>

extern int bf_main(char *tape);

int main() {
    char *tape = calloc(30000, 1);
    if (!tape) return 1;
    bf_main(tape);
    free(tape);
    return 0;
}
