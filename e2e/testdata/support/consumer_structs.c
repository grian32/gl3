#include <stdint.h>
struct Small { int32_t x, y; };
struct Mixed { float fraction; int32_t count; };
struct Large { int64_t first, second, third; };
extern struct Small pass_small(struct Small);
extern struct Mixed pass_mixed(struct Mixed);
extern struct Large pass_large(struct Large);
int main(void) {
    struct Small a = pass_small((struct Small){17, -42});
    if (a.x != 17 || a.y != -42) return 1;
    struct Mixed b = pass_mixed((struct Mixed){1.5f, 73});
    if (b.fraction != 1.5f || b.count != 73) return 2;
    struct Large c = pass_large((struct Large){INT64_C(5000000000), -19, 42});
    if (c.first != INT64_C(5000000000) || c.second != -19 || c.third != 42) return 3;
    return 0;
}
