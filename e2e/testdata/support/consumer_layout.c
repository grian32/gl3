#include <stdint.h>
struct Padded { int8_t tag; int32_t value; int8_t tail; };
struct Nested { int8_t prefix; struct Padded inner; int16_t suffix; };
extern uint64_t padded_size(void);
extern uint64_t nested_size(void);
int main(void) {
    if (padded_size() != sizeof(struct Padded)) return 1;
    if (nested_size() != sizeof(struct Nested)) return 2;
    return 0;
}
