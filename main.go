package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"harish.com/keenbase/keenbase"
)

func main() {

	serveCmd := flag.NewFlagSet("serve", flag.ExitOnError)
	servePort := serveCmd.Int("port", 8090, "HTTP port to listen on")
	serveData := serveCmd.String("data", "./pb_data", "data directory for the SQLite database and uploads")

	superuserCmd := flag.NewFlagSet("superuser", flag.ExitOnError)
	superuserEmail := superuserCmd.String("email", "", "email of the superuser to create (required)")
	superuserPass := superuserCmd.String("password", "", "password of the superuser to create (required, min 8 chars)")
	superuserData := superuserCmd.String("data", "./pb_data", "data directory used by the database")

	usage := func() {
		fmt.Fprintln(os.Stderr, `Usage:
  keenbase serve [flags]
    --port  int     HTTP port (default 8090)
    --data  string  data directory (default ./pb_data)

  keenbase superuser [flags]
    --email    string  superuser email (required)
    --password string  superuser password (required)
    --data     string  data directory (default ./pb_data)
`)
	}

	if len(os.Args) < 2 {
		// Default: just start the server.
		runServe(8090, "./pb_data")
		return
	}

	switch os.Args[1] {
	case "serve":
		if err := serveCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		runServe(*servePort, *serveData)

	case "superuser":
		if err := superuserCmd.Parse(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		if *superuserEmail == "" || *superuserPass == "" {
			fmt.Fprintln(os.Stderr, "error: --email and --password are required")
			superuserCmd.Usage()
			os.Exit(1)
		}
		runCreateSuperuser(*superuserEmail, *superuserPass, *superuserData)

	default:
		usage()
		os.Exit(1)
	}
}

func runServe(port int, dataDir string) {
	sb := keenbase.WithConfig(keenbase.Config{Port: port, DataDir: dataDir})
	if err := sb.Start(); err != nil {
		log.Fatal(err)
	}
}

func runCreateSuperuser(email, password, dataDir string) {
	sb := keenbase.WithConfig(keenbase.Config{DataDir: dataDir})
	if err := sb.CreateSuperuser(email, password); err != nil {
		log.Fatalf("failed to create superuser: %v", err)
	}
	fmt.Printf("Superuser %q created successfully.\n", email)
}
