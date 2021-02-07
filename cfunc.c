#include "cfunc.h"

#include <stdio.h>
#include <string.h>
#include <unistd.h>

static const char expected[] = "hello world bytes";

int cfunc_uintptr(uintptr_t data, size_t length) { return cfunc_void((const void*)data, length); }

int cfunc_void(const void* data, size_t length) {
  // -1 to remove the null terminator
  if (length != sizeof(expected) - 1) {
    printf("unexpected size: %zu != %zu\n", length, sizeof(expected));
    return 1;
  }
  if (memcmp(data, expected, length) != 0) {
    printf("data=%p ERROR: unexpected bytes before sleep\n", data);
    return 1;
  }
  sleep(1);
  if (memcmp(data, expected, length) != 0) {
    printf("data=%p ERROR: unexpected bytes after sleep\n", data);
    return 1;
  }
  return 0;
}

void cfunc_many_uintptr(uintptr_t data1, size_t length1, uintptr_t data2, size_t length2,
                        uintptr_t data3, size_t length3) {
  cfunc_void((const void*)data1, length1);
  cfunc_void((const void*)data2, length2);
  cfunc_void((const void*)data3, length3);
}
