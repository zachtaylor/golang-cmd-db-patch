# `db-patch`

`go get taylz.io/cmd/db-patch`

Connect to a database using MySQL, and execute a series of patches

Patches are contained separately in files, known as patch files. These files
- contain SQL statements, which are executed as transactions (each patch will succeed or fail as a whole)
- begin with 4 numbers, identifying the patch number in sequence
- end with ".sql"
