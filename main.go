package main

import "fmt"

func main() {
	t := NewART()
	t.Insert("foo", 1)
	t.Insert("far", 2)
	t.Insert("fooz", 3)
	t.Insert("faz", 4)
	fmt.Println(t.Search("far"))

}
