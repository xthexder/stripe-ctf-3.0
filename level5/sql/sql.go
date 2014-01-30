package sql

import (
	"regexp"
	"strconv"
)

type SQL struct {
	path           string
	sequenceNumber int
	//mutex          sync.Mutex
	regex *regexp.Regexp
	ctf   map[string]*Ctf3
}

type Ctf3 struct {
	name         string
	friendCount  int
	requestCount int
	favoriteWord string
}

func NewSQL(path string) *SQL {
	reg, _ := regexp.Compile("^UPDATE ctf3 SET friendCount=friendCount\\+([0-9]+), requestCount=requestCount\\+1, favoriteWord=\"([^\"]+)\" WHERE name=\"([^\"]+)\"; SELECT \\* FROM ctf3;$")
	sql := &SQL{
		path:  path,
		regex: reg,
		ctf:   make(map[string]*Ctf3),
	}
	return sql
}

func (ctf *Ctf3) String() string {
	return ctf.name + "|" + strconv.Itoa(ctf.friendCount) + "|" + strconv.Itoa(ctf.requestCount) + "|" + ctf.favoriteWord + "\n"
	//return fmt.Sprintf("%s|%d|%d|%s\n", ctf.name, ctf.friendCount, ctf.requestCount, ctf.favoriteWord)
}

func (sql *SQL) Execute(command string) (string, int) {
	defer func() { sql.sequenceNumber += 1 }()
	//log.Printf("[%d] Executing %#v", sql.sequenceNumber, command)
	if sql.sequenceNumber == 0 {
		for _, name := range []string{"siddarth", "gdb", "christian", "andy", "carl"} {
			sql.ctf[name] = &Ctf3{name, 0, 0, ""}
		}
		//log.Printf("[%d] Table created: %s", sql.sequenceNumber, command)
		return "", sql.sequenceNumber
	} else {
		matches := sql.regex.FindStringSubmatch(command)
		ctf := sql.ctf[matches[3]]
		friendInc, _ := strconv.Atoi(matches[1])
		ctf.friendCount += friendInc
		ctf.requestCount++
		ctf.favoriteWord = matches[2]
		return sql.ctf["siddarth"].String() + sql.ctf["gdb"].String() + sql.ctf["christian"].String() + sql.ctf["andy"].String() + sql.ctf["carl"].String(), sql.sequenceNumber
	}
}
