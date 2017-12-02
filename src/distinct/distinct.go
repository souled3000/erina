package main

import (
	"crypto/md5"
	//	"encoding/hex"
	"fmt"
	"os"
	"path/filePath"
)

var (
	m   = make(map[[16]byte]string, 100)
	buf = make([]byte, 1024)
	e   error
	x   int
)

func getCurrentDirectory() string {
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return dir
}
func main() {
	var pr string
	if len(os.Args) == 1 {
		pr = getCurrentDirectory()
	}
	f, e := os.Open(pr)
	if e == nil {
		finfo, _ := f.Stat()
		if finfo.IsDir() {
			recur(pr)
		} else {
			fmt.Println("we need a Folder!!")
		}
	} else {
		fmt.Errorf("ERR: %v\n", e)
	}
}

func recur(path string) {
	f, e := os.Open(path)
	defer f.Close()
	// a list of files in the folder 'f'
	names, _ := f.Readdirnames(-1)
	// go iterate the list
	for _, name := range names {
		p := path + string(os.PathSeparator) + name
		f, _ = os.Open(p)
		finfo, _ := f.Stat()
		if finfo.IsDir() {
			f.Close()
			recur(p)
		}
		_, e = f.Read(buf)
		if e != nil {
			fmt.Errorf("ERR:%v\n", e)
			continue
		}

		// calulate md5 of each file
		fmd5 := md5.Sum(buf)
		//		fmt.Printf("%s\t:\t%s\n", hex.EncodeToString(buf[1000:1005]), hex.EncodeToString(fmd5[:]))
		// judge if the md5 has been in the map 'm'
		// if it has been existed, deleting the file, otherwise storing the md5 into the map 'm'
		if v, ok := m[fmd5]; ok {
			x++
			fmt.Printf("%s = %s del %s %d\n", v, name, p, x)
			f.Close()
			os.Remove(p)
		} else {
			m[fmd5] = name
		}
		f.Close()
	}
}
