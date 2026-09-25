#include <stdint.h>
extern int32_t identity(int32_t value);
int main(void) { return identity(42) == 42 ? 0 : 1; }
