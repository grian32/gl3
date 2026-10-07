all:
	go build -tags=llvm22 -o gl3 .

clean:
	rm -f gl3
