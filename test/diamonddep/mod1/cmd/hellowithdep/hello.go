// hellowithdep has an internal and external dependency, as well as an external dependency that depends on internal code.
package main

import (
	"fmt"

	"github.com/u-root/gobusybox/test/diamonddep/mod1/pkg/hello"
	"github.com/u-root/gobusybox/test/diamonddep/mod2/pkg/exthello"
	hello2 "github.com/u-root/gobusybox/test/diamonddep/mod2/pkg/hello"
)

func main() {
	fmt.Printf("test/diamonddep/mod1/hello: %s\n", hello.Hello())
	fmt.Printf("test/diamonddep/mod2/hello: %s\n", hello2.Hello())
	fmt.Printf("test/diamonddep/mod2/exthello: %s\n", exthello.Hello())
}
