//go:generate go run .
package main

import (
	"log"
	"os"
	"text/template"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/mimc"
)

// Declare type pointer to a template
var temp *template.Template

var funcs = template.FuncMap{
	"slice": func(arr []string, start int, end int) []string {
		return arr[start:end]
	},
	"sub": func(a, b int) int {
		return a - b
	},
}

// Using the init function to make sure the template is only parsed once in the program
func init() {
	// template.Must takes the reponse of template.ParseFiles and does error checking
	temp = template.Must(template.New("mimc.sol.gotmpl").Funcs(funcs).ParseGlob("./mimc.sol.gotmpl"))
}

var Constants = func() []string {
	res := make([]string, len(mimc.GetConstants()))
	for i := range res {
		res[i] = mimc.GetConstants()[i].String()
	}
	return res
}()

func main() {
	constants := make([]string, len(mimc.GetConstants()))
	for i := range constants {
		constants[i] = mimc.GetConstants()[i].String()
	}

	f, err := os.Create("../../../../contracts/src/libraries/Mimc.sol")
	if err != nil {
		log.Fatal("cannot create file: ", err)
		return
	}
	err = temp.Execute(f, constants)
	if err != nil {
		log.Fatal("cannot execute templating: ", err)
		return
	}

	f.Close()
}
