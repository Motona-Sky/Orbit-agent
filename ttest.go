package main

import (
	"fmt"
	mem "looporbit/internal/memorys"
)

func main() {
	fmt.Println(mem.GetUserMemorysPrompt())
	mem.DelUserMemorys(7)
}
