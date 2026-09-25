#include <stdbool.h>
#include <stdint.h>
#include <stddef.h>

extern int8_t signed8(int8_t);
extern int16_t signed16(int16_t);
extern int32_t signed32(int32_t);
extern int64_t signed64(int64_t);
extern uint8_t unsigned8(uint8_t);
extern uint16_t unsigned16(uint16_t);
extern uint32_t unsigned32(uint32_t);
extern uint64_t unsigned64(uint64_t);
extern float floating(float);
extern bool boolean(bool);
extern char character(char);
extern void *pointer(void *);
extern void nothing(void);

int main(void) {
    if (signed8(INT8_MIN) != INT8_MIN) return 1;
    if (signed16(INT16_MIN) != INT16_MIN) return 2;
    if (signed32(INT32_MIN) != INT32_MIN) return 3;
    if (signed64(INT64_MIN) != INT64_MIN) return 4;
    if (unsigned8(UINT8_MAX) != UINT8_MAX) return 5;
    if (unsigned16(UINT16_MAX) != UINT16_MAX) return 6;
    if (unsigned32(UINT32_MAX) != UINT32_MAX) return 7;
    if (unsigned64(UINT64_MAX) != UINT64_MAX) return 8;
    if (floating(-1.5f) != -1.5f) return 9;
    if (!boolean(true) || boolean(false)) return 10;
    if (character('Z') != 'Z') return 11;
    int32_t value = 42;
    if (pointer(&value) != &value || pointer(NULL) != NULL) return 12;
    nothing();
    return 0;
}
