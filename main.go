package main

import "fmt"

func main() {
	t := NewART()
	t.Insert("car", "vehicle")
	t.Insert("ca", "short")
	fmt.Println(t.Search("ca"))

}
