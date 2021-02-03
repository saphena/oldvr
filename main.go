package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"time"

	"github.com/pkg/browser"

	_ "github.com/mattn/go-sqlite3"
)

var (
	debug    = flag.Bool("debug", false, "enable debugging")
	port     = flag.String("port", "80", "the HTTP port")
	db3      = flag.String("db", "cdrs.db", "full path to db")
	starturl = flag.String("url", "http://localhost", "starting URL")
	pagesz   = flag.Int("pg", 15, "Pagesize for results")
)

const myversion = "v0.4"

var sqldb *sql.DB

func formatCommas(num int) string {
	str := fmt.Sprintf("%d", num)
	re := regexp.MustCompile("(\\d+)(\\d{3})")
	for n := ""; n != str; {
		n = str
		str = re.ReplaceAllString(str, "$1,$2")
	}
	return str
}

func fetchTemplate(template string) string {

	file, err := os.Open(template)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err = file.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	b, err := ioutil.ReadAll(file)
	return string(b)
}

func getValueFromDB(sql, colname, defvalue string) string {

	row, err := sqldb.Query(sql)
	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()
	var res string = defvalue
	if row.Next() {
		var rex string
		row.Scan(&rex)
		res = rex
	}
	return res

}

func showDuration(duration string) string {

	var re = regexp.MustCompile(`\:`)
	var res string = ""
	hms := re.Split(duration, 3)
	h, _ := strconv.Atoi(hms[0])
	m, _ := strconv.Atoi(hms[1])
	s, _ := strconv.Atoi(hms[2])
	if h != 0 {
		res += strconv.Itoa(h) + "h "
		res += strconv.Itoa(m) + "m "
	} else if m != 0 {
		res += strconv.Itoa(m) + "m "
	}
	if s != 0 {
		res += strconv.Itoa(s) + "s "
	}
	return res
}

func showDate(dt string) string {

	layout := "2006-01-02"
	t, _ := time.Parse(layout, dt)

	return t.Format("2 Jan 2006")
}

func showDatetime(dt string) string {

	layout := "2006-01-02T15:04:05Z"
	t, _ := time.Parse(layout, dt)

	return t.Format("2 Jan 2006 3:04pm Mon")
}

func countrows(table, where string) int {

	var sql = "SELECT Count(1) As rex FROM " + table + " WHERE " + where
	var res int
	res, _ = strconv.Atoi(getValueFromDB(sql, "rex", "-1"))
	return res

}

func lookupHandler(w http.ResponseWriter, r *http.Request) {

	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(w, "ParseForm() err: %v", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, fetchTemplate("htmlhead.html"))
	fmt.Fprintf(w, "<h2>%v</h2>", getValueFromDB("SELECT dbname FROM params", "dbname", ""))
	fmt.Fprintf(w, fetchTemplate("htmllookup.html"))

	var tel string = r.FormValue("tel")
	var daterange bool = r.FormValue("dates") != "all"
	var fromdate string = r.FormValue(("fromdate"))
	var todate string = r.FormValue("todate")
	var offset int = 0

	offset, _ = strconv.Atoi(r.FormValue("offset"))

	fmt.Fprintf(w, "<p>Showing ")
	if tel != "" {
		fmt.Fprintf(w, "%v", tel)
	} else {
		fmt.Fprintf(w, "all calls")
	}
	if daterange {
		fmt.Fprintf(w, "; Call date ")
		if fromdate == "" {
			fmt.Fprintf(w, "upto "+showDate(todate))
		} else {
			fmt.Fprintf(w, showDate(fromdate))
			if todate != "" {
				if todate != fromdate {
					fmt.Fprintf(w, " - "+showDate(todate))
				}
			} else {
				fmt.Fprintf(w, " onwards")
			}
		}
	}

	if todate != "" {
		todate += "T23:59:59"
	}

	var sql string = "SELECT cdrid,direction,duration,connected,ifnull(aphone,'') as aphone,ifnull(bphone,'') as bphone,folderid "
	var where string = ""
	sql += "FROM cdrs "
	sql += "WHERE "
	if tel != "" {
		where += "(aphone LIKE '%" + tel + "%' OR bphone LIKE '%" + tel + "%') "
	} else {
		where += "1=1 "
	}
	if daterange {
		if fromdate != "" {
			where += " AND connected >= '" + fromdate + "'"
		}
		if todate != "" {
			where += " AND connected <= '" + todate + "'"
		}
	}
	nresults := countrows("cdrs", where)

	fmt.Fprintf(w, "; %v found</p>", formatCommas(nresults))

	sql += where
	sql += " LIMIT " + strconv.Itoa(offset) + ", " + strconv.Itoa(*pagesz)

	fmt.Fprintf(w, "\n<!-- %v -->\n", sql)

	rows, err := sqldb.Query(sql)
	if err != nil {
		fmt.Fprintf(w, "OMG!!! %v", err)
		return
	}
	fmt.Fprintf(w, "<table id=\"results\"><thead><tr>")
	fmt.Fprintf(w, "<th class=\"duration\">Duration</th>")
	fmt.Fprintf(w, "<th class=\"connected\">Connected</th>")
	fmt.Fprintf(w, "<th class=\"direction\">I/O</th>")
	fmt.Fprintf(w, "<th class=\"aphone\">From</th>")
	fmt.Fprintf(w, "<th class=\"bphone\">To</th></tr></thead><tbody>")
	for rows.Next() {
		var cdrid string
		var direction string
		var duration string
		var connected string
		var aphone, bphone string
		var folderid int
		rows.Scan(&cdrid, &direction, &duration, &connected, &aphone, &bphone, &folderid)
		var record string = "/cdr" + strconv.Itoa(folderid) + "/{" + cdrid + "}.osf"
		fmt.Fprintf(w, "<tr><td class=\"duration\">%v</td>", showDuration(duration))
		fmt.Fprintf(w, "<td class=\"connected\">%v</td>", showDatetime(connected))
		fmt.Fprintf(w, "<td class=\"direction\">%v</td>", direction)
		fmt.Fprintf(w, "<td class=\"aphone\">%v</td><td class=\"bphone\">%v</td>", aphone, bphone)
		fmt.Fprintf(w, "<td class=\"audio\"><audio controls><source src=")
		fmt.Fprintf(w, "\"%v\" type=\"audio/mpeg\"></audio></td>", record)
		fmt.Fprintf(w, "</tr>")
	}
	fmt.Fprintf(w, "</tbody></table>")

	if offset > 0 {
		fmt.Fprintf(w, "<form action=\"lookup\" method=\"post\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"tel\" value=\""+r.FormValue("tel")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"dates\" value=\""+r.FormValue("dates")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"fromdate\" value=\""+r.FormValue("fromdate")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"todate\" value=\""+r.FormValue("todate")+"\">")
		var poffset int = 0
		if offset >= *pagesz {
			poffset = offset - *pagesz
		}
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"offset\" value=\""+strconv.Itoa(poffset)+"\">")
		fmt.Fprintf(w, "<input type=\"submit\" value=\"&NestedLessLess;\"> ")
		fmt.Fprintf(w, "</form>")

	}

	if nresults-offset > *pagesz {
		fmt.Fprintf(w, "<form action=\"lookup\" method=\"post\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"tel\" value=\""+r.FormValue("tel")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"dates\" value=\""+r.FormValue("dates")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"fromdate\" value=\""+r.FormValue("fromdate")+"\">")
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"todate\" value=\""+r.FormValue("todate")+"\">")
		var poffset int = offset + *pagesz
		fmt.Fprintf(w, "<input type=\"hidden\" name=\"offset\" value=\""+strconv.Itoa(poffset)+"\">")
		fmt.Fprintf(w, "<input type=\"submit\" value=\"&NestedGreaterGreater;\"> ")
		fmt.Fprintf(w, "</form>")

	}

}

func startServer() {

	var numrex int
	row, err := sqldb.Query("SELECT Count(1) As rex FROM cdrs")
	if err != nil {
		log.Fatal(err)
	}
	if row.Next() {
		row.Scan(&numrex)
		fmt.Printf("Number of CDRs: %v\n", formatCommas(numrex))
	}
	row.Close()
	browser.OpenURL(*starturl + ":" + *port)

}

func handleFolder(folderid int, datapath string) {

	cdrserver := http.FileServer(http.Dir(datapath))
	cdrf := "/cdr" + strconv.Itoa(folderid) + "/"
	http.Handle(cdrf, http.StripPrefix(cdrf, cdrserver))
	fmt.Printf("VR%v: %v\n", folderid, datapath)

}

func handleFolders() {

	rows, err := sqldb.Query("SELECT folderid,datapath FROM folders")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var folderid int
		var datapath string
		rows.Scan(&folderid, &datapath)
		handleFolder(folderid, datapath)
	}

}

func configHandler(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	fmt.Fprintf(w, fetchTemplate("htmlhead.html"))

	if err := r.ParseForm(); err == nil {

		var x, y string
		var updated bool = false

		x = r.FormValue("dbname")
		if x != "" {
			var sql string = "UPDATE params SET dbname=?"
			sqldb.Exec(sql, x)
			updated = true
		}
		x = r.FormValue("datapath")
		y = r.FormValue("folderid")
		if x != "" {
			var sql string = "UPDATE folders SET datapath=? WHERE folderid=?"
			sqldb.Exec(sql, x, y)
			updated = true
		}

		if updated {
			fmt.Fprintf(w, "<h2>%v</h2>", getValueFromDB("SELECT dbname FROM params", "dbname", ""))
			fmt.Fprintf(w, fetchTemplate("htmllookup.html"))
			return
		}
	}

	var dbname string = getValueFromDB("SELECT dbname FROM params", "dbname", "*unknown*")

	fmt.Fprintf(w, "<h2>Database configuration</h2>")

	fmt.Fprintf(w, "<div id=\"dbconfig\">")
	fmt.Fprintf(w, "<form action=\"config\" method=\"post\">")
	fmt.Fprintf(w, "<label for=\"dbname\">DB description: </label>")
	fmt.Fprintf(w, "<input type=\"text\" id=\"dbname\" name=\"dbname\" value=\"%v\">", dbname)

	row, err := sqldb.Query("SELECT folderid,datapath FROM folders ORDER BY folderid")
	if err != nil {
		log.Fatal(err)
	}
	defer row.Close()

	fmt.Fprintf(w, "<p>Folders containing voice recordings</p>")

	fmt.Fprintf(w, "<ul id=\"folderlist\">")
	for row.Next() {
		var folderid int
		var datapath string
		row.Scan(&folderid, &datapath)
		fmt.Fprintf(w, "<li><input type=\"text\" name=\"folderid\" value=\"%v\" readonly> : ", folderid)
		fmt.Fprintf(w, "<input type=\"text\" name=\"datapath\" value=\"%v\"></li>", datapath)

	}
	fmt.Fprintf(w, "</ul>")

	fmt.Fprintf(w, "<input type=\"submit\" value=\"Update\">")
	fmt.Fprintf(w, "</form>")
	fmt.Fprintf(w, "</div>")

}

func main() {

	fmt.Printf("\nOldVRs %v\nCopyright (c) Bob Stammers 2021\n", myversion)
	fmt.Printf("Architecture: %v\n\n", runtime.GOARCH)

	flag.Parse()

	var err error
	sqldb, err = sql.Open("sqlite3", *db3)
	if err != nil {
		log.Fatal(err)
	}
	defer sqldb.Close()

	fmt.Printf("CDRs: %v\n", *db3)
	fmt.Printf("Database: %v\n", getValueFromDB("SELECT dbname FROM params", "dbname", "unidentified"))
	fileServer := http.FileServer(http.Dir("."))
	http.Handle("/", fileServer)

	http.HandleFunc("/lookup", lookupHandler)
	http.HandleFunc("/config", configHandler)

	handleFolders()

	go startServer()

	fmt.Printf("Serving port " + *port + "\n")
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatal(err)
	}
}
