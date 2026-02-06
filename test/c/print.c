#include <stdio.h>

void print_int(int x) { printf("Result: %d\n", x); }

void print_str(char *s) { printf("%s", s); }

void print_float(double d) { printf("Float: %f\n", d); }

// Struct test helpers
typedef struct {
  int x;
  int y;
} Point;
void print_point(Point p) { printf("Point: (%d, %d)\n", p.x, p.y); }

typedef struct {
  double x;
  double y;
} Vec2;
void print_vec2(Vec2 v) { printf("Vec2: (%f, %f)\n", v.x, v.y); }
