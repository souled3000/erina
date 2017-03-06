package srv

import (
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

func getDevIdByMac(mac string) (a int64, b string, err error) {
	db, err := sql.Open("mysql", MYSQL)
	defer db.Close()
	sql := "select id,mac_match from s_device where mac_match = ?"
	row := db.QueryRow(sql, mac)
	err = row.Scan(&a, &b)
	return
}
