// Example:
//
//import (
//	"fmt"
//	"time"
//
//	stat "github.com/c9s/goprocinfo/linux"
//)
//
//func main() {
//	ch := NewWatcher(time.Second)
//	n := 0
//	for f := range ch {
//		if f.Error != nil {
//			fmt.Printf("error: %v", f.Error)
//			break
//		}
//		n++
//		fmt.Printf("CPU usage: %f\n", f.Usage * 100.0)
//		if n > 10 {
//			break
//		}
//	}
//	close(ch)
//}

package procinfo
