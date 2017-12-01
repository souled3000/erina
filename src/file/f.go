package main

import (
	"fmt"
	"os"
)

func main() {
	fn := "c:\\users\\juliana\\test.f"
	f, _ := os.Create(fn)
	finfo, _ := f.Stat()
	fmt.Printf("filename: %s\n", finfo.Name())
	f.Close()
	os.Remove(fn)

	fn = "c:\\users\\juliana\\testdir"
	os.MkdirAll(fn, 0777)
	f, _ = os.Open(fn)

	finfo, _ = f.Stat()
	fmt.Printf("filename: %s\n", finfo.Name())
	f.Close()
	os.RemoveAll(fn)

}
