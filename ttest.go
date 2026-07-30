package main

import (
	"fmt"
	mem "looporbit/internal/memorys"
)

func main() {
	for {
		mem.DelUserMemorys(1)
		fmt.Print(mem.GetUserMemorysPrompt())
	}

}
