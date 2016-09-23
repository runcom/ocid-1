src = $(wildcard *.c)
obj = $(src:.c=.o)

LIBS =
CFLAGS = -Wall -Wextra -std=c99

conmon: $(obj)
	$(CC) -o $@ $^ $(LIBS)

.PHONY: clean
clean:
	rm -f $(obj) conmon
