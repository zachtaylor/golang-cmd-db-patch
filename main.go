package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"taylz.io/db"
	"taylz.io/db/mysql"
	"taylz.io/db/patch"
	"taylz.io/env"
	"taylz.io/log"
)

const (
	version  = "v0.0.0"
	dbUser   = "DB_USER"
	dbPswd   = "DB_PASSWORD"
	dbHost   = "DB_HOST"
	dbPort   = "DB_PORT"
	dbName   = "DB_NAME"
	patchDir = "PATCH_DIR"
)

// HelpMessage is printed when you use arg "-help" or -"h"
var HelpMessage = `
	db-patch runs sequential .sql files as transactions
	internally uses (or can create) table "patch" to manage revision number

	--- options
	[name]			[default]			[comment]

	version			false				print the version number

	-help, -h		false				print this help page and then quit

	-PATCH_DIR		"./"				directory to load patch files from

	-DB_USER		(required)			username to use when connecting to database

	-DB_PASSWORD		(required)			password to use when conencting to database

	-DB_HOST		(required)			database host ip address

	-DB_PORT		(required)			port to open database host ip with mysql

	-DB_NAME		(required)			database name to connect to within database server
`

func newenv() env.Values {
	return env.Values{
		dbUser: "",
		dbPswd: "",
		dbHost: "",
		dbPort: "",
		dbName: "",
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("taylz.io/cmd/db-patch@" + version)
	}

	env, _ := newenv().ParseDefault()

	if env["help"] == "true" || env["h"] == "true" {
		fmt.Print(HelpMessage)
		return
	}

	logger := log.Default()

	logger.With(log.Fields{
		dbName:   env[dbName],
		patchDir: env[patchDir],
	}).Debug("db-patch")

	conn, err := mysql.Open(db.DSN(env[dbUser], env[dbPswd], env[dbHost], env[dbPort], env[dbName]))
	if conn == nil {
		logger.Add("Error", err).Error("failed to open db")
		return
	}
	logger.With(log.Fields{
		dbHost: env[dbHost],
		dbName: env[dbName],
	}).Info("opened connection")

	// get current patch info
	pid, err := patch.Get(conn)
	if err == db.ErrPatchTable {
		logger.Warn(err.Error())
		ansbuf := "?"
		for ansbuf != "y" && ansbuf != "" && ansbuf != "n" {
			fmt.Print(`patch table does not exist, create patch table now? (y/n): `)
			fmt.Scanln(&ansbuf)
			ansbuf = strings.Trim(ansbuf, " \t")
		}
		if ansbuf == "n" {
			logger.Info("exit without creating patch table")
			return
		}
		if err := mysql.CreatePatchTable(conn); err != nil {
			logger.Add("Error", err).Error("failed to create patch table")
			return
		}
		logger.Info("created patch table")
		pid = 0 // reset pid=-1 from the error state
	} else if err != nil {
		logger.Add("Error", err).Error("failed to identify patch number")
		return
	} else {
		logger.Info("found patch#", pid)
	}

	patches := patch.GetFiles(env["PATCH_DIR"])
	if len(patches) < 1 {
		logger.Error("no patches found")
		return
	}

	for i := pid + 1; patches[i] != ""; i++ {
		logger.Trace("queue patch#", i, " 	file:", patches[i])
	}

	// ask about patches
	ansbuf := "?"
	for ansbuf != "y" && ansbuf != "" && ansbuf != "n" {
		fmt.Print("Apply patches? [Y/n]: ")
		fmt.Scanln(&ansbuf)
		ansbuf = strings.Trim(ansbuf, " \t\n")
	}
	if ansbuf == "n" {
		logger.Info("not applying patches")
		return
	}

	// apply patches
	for pid++; patches[pid] != ""; pid++ {
		pf := patches[pid]
		log := logger.With(log.Fields{
			"PatchID":   pid,
			"PatchFile": pf,
		})
		tStart := time.Now()
		if sql, err := ioutil.ReadFile(pf); err != nil {
			log.Add("Error", err).Error("failed to read patch file")
			return
		} else if err = db.ExecTx(conn, string(sql)); err != nil {
			log.Add("Error", err).Error("failed to patch")
			return
		} else if _, err = conn.Exec("UPDATE patch SET patch=?", pid); err != nil {
			log.Add("Error", err).Error("failed to update patch number")
			return
		}
		log.Add("Time", time.Since(tStart)).Info("applied patch")
	}

	logger.Add("Patch", pid-1).Info("done")
}

func ENV() env.Values {
	return env.Values{
		"PATCH_DIR": "",
	}
}
