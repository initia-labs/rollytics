package main

func main() {
	rootCmd := NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
