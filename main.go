package main

import "fmt"

func main() {
	t := NewART()
	t.Insert("cat", "animal")
	t.Insert("cats", "plural")
	fmt.Println(t.Search("cats"))

}
