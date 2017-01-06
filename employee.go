package main

import (
	"database/sql"
	"gopkg.in/gorp.v1"
	"strconv"
    "time"
	"reflect"
	"os"
	log "github.com/Sirupsen/logrus"
	"flag"
	"strings"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"fmt"
	"github.com/BurntSushi/toml"
)

type tomlConfig struct {
    Title   string
    Owner   ownerInfo
    DB      database `toml:"database"`
    Servers server `toml:"server"`
    Clients clients
	Port	string
}

type ownerInfo struct {
    Name    string
    Org     string
    Bio     string
    DOB     time.Time
}

type database struct {
    Server  string
    Ports   []int
    ConnMax int `toml:"connection_max"`
    Enabled bool
}

type server struct {
	Port   string
}

type clients struct {
    Data    [][]interface{}
    Hosts   []string
}

type Employee struct {
	Id			int64  `db:"id" json:"id"`
	Firstname	string `db:"firstname" json:"firstname"`
	Lastname	string `db:"lastname" json:"lastname"`
    Doj			time.Time `db:"doj" json:"doj"`
	Skills		string `db:"skills" json:"skills"`
}


var dbmap = initDb()

func initDb() *gorp.DbMap {
	
	log.SetFormatter(&log.JSONFormatter{})
	log.SetOutput(os.Stderr)
	log.SetLevel(log.WarnLevel)

	// Database configuration
	db, err := sql.Open("mysql", "root:root@/ranga?charset=utf8&parseTime=true")
	checkErr(err, "sql.Open failed")
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.MySQLDialect{"InnoDB", "UTF8"}}
	
	// Table create
	dbmap.AddTableWithName(Employee{}, "Employee").SetKeys(true, "id")
	err = dbmap.CreateTablesIfNotExists()
	checkErr(err, "Create tables failed")

	return dbmap
}

func checkErr(err error, msg string) {
	if err != nil {
		log.Fatalln(msg, err)
	}
}

func Cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
		c.Next()
	}
}

func main() {
	var config tomlConfig
	if _, err := toml.DecodeFile("employee.toml", &config); err != nil {
        fmt.Println(err)
        return
    }

	r := gin.Default()
	
	r.Use(Cors())

	v1 := r.Group("api/v1")
	{
		v1.GET("/emps", GetEmployees)
		v1.GET("/emps/:id", GetEmployee)
		v1.POST("/emps", PostEmployee)
		v1.PUT("/emps/:id", UpdateEmployee)
		v1.DELETE("/emps/:id", DeleteEmployee)
		v1.OPTIONS("/emps", OptionsEmployee)     // POST
		v1.OPTIONS("/emps/:id", OptionsEmployee) // PUT, DELETE
	}

	fmt.Printf("Title: %s\n", config.Title)
	fmt.Printf("Owner: %s (%s, %s), Born: %s\n",
        config.Owner.Name, config.Owner.Org, config.Owner.Bio,
        config.Owner.DOB)
	
	portPtr := flag.String("p", config.Port, "Port number")
	fmt.Printf("Client data: %v\n", config.Clients.Data)
    fmt.Printf("Client hosts: %5v\n", config.Clients.Hosts)
	flag.Parse()
	r.Run(":" + *portPtr)
}

// Fetching all employee data
func GetEmployees(c *gin.Context) {
	var emps []Employee
	_, err := dbmap.Select(&emps, "SELECT id, firstname, lastname, doj, skills FROM employee")
	
	if err == nil {
		c.JSON(200, emps)
	} else {
		c.JSON(404, gin.H{"error": "no employee(s) into the table"})
	}

	// curl -i http://localhost:9090/api/v1/emps
}

// Fetching employee data by id
func GetEmployee(c *gin.Context) {
	id := c.Params.ByName("id")
	var emp Employee
	err := dbmap.SelectOne(&emp, "SELECT * FROM employee WHERE id=? LIMIT 1", id)
	
	if err == nil {
		emp_id, _ := strconv.ParseInt(id, 0, 64)

		content := &Employee{
			Id:			emp_id,
			Firstname:	emp.Firstname,
			Lastname:	emp.Lastname,
			Doj:		emp.Doj,
			Skills:		emp.Skills,
		}
		c.JSON(200, content)
		log.Print(content)
	} else {
		c.JSON(404, gin.H{"error": "employee not found"})
	}

	// curl -i http://localhost:8080/api/v1/users/1
}

// Creating new employee record
func PostEmployee(c *gin.Context) {
	var emp Employee
	c.Bind(&emp)
	
	skills := strings.Replace(emp.Skills, "'", "\\'", -1)
	fmt.Println(reflect.TypeOf(emp.Skills))
	
	if emp.Firstname != "" && emp.Lastname != "" {

		if insert, _ := dbmap.Exec(`INSERT INTO employee (firstname, lastname, doj, skills) VALUES (?, ?, ?, ?)`, emp.Firstname, emp.Lastname, emp.Doj, skills); insert != nil {
			emp_id, err := insert.LastInsertId()
			log.Println(err)
			log.Println(emp_id)
			if err == nil {
				content := &Employee{
					Id:         emp_id,
					Firstname:  emp.Firstname,
					Lastname:   emp.Lastname,
                    Doj:        emp.Doj,
					Skills:		emp.Skills,
				}
				c.JSON(201, content)
			} else {
				checkErr(err, "Insert failed")
			}
		}
	} else {
		c.JSON(400, gin.H{"error": "Fields are empty"})
	}

	// curl -i -X POST -H "Content-Type: application/json" -d "{ \"firstname\": \"Thea\", \"lastname\": \"Queen\", \"doj\": \"2014-10-19T23:08:24Z\", \"skills\": \"["Go", "C","Ruby"]\" }" http://localhost:9090/api/v1/emps
}

// Update existing employee record
func UpdateEmployee(c *gin.Context) {
	id := c.Params.ByName("id")
	var emp Employee
	err := dbmap.SelectOne(&emp, "SELECT * FROM employee WHERE id=?", id)

	if err == nil {
		var js Employee
		c.Bind(&js)
		emp_id, _ := strconv.ParseInt(id, 0, 64)

		employee := Employee{
			Id:         emp_id,
			Firstname:  js.Firstname,
			Lastname:   js.Lastname,
            Doj:        js.Doj,
			Skills:		js.Skills,
		}

		if employee.Firstname != "" && employee.Lastname != "" {
			_, err = dbmap.Update(&employee)
			log.Println(err)
			if err == nil {
				c.JSON(200, employee)
			} else {
				checkErr(err, "Updated failed")
			}
		} else {
			c.JSON(400, gin.H{"error": "fields are empty"})
		}
	} else {
		c.JSON(404, gin.H{"error": "employee not found"})
	}
	// curl -i -X PUT -H "Content-Type: application/json" -d "{ \"firstname\": \"Thea\", \"lastname\": \"Merlyn\", \"doj\": \"2011-10-19T23:08:24Z\", \"skills\": \"["C", Go", "Rails","Ruby"]\" }" http://localhost:9090/api/v1/emps/1
}


// Delete employee record by id
func DeleteEmployee(c *gin.Context) {
	id := c.Params.ByName("id")

	var emp Employee
	err := dbmap.SelectOne(&emp, "SELECT * FROM employee WHERE id=?", id)

	if err == nil {
		_, err = dbmap.Delete(&emp)

		log.Println(err)
		if err == nil {
			c.JSON(200, gin.H{"id #" + id: "deleted"})
		} else {
			checkErr(err, "Delete failed")
		}

	} else {
		c.JSON(404, gin.H{"error": "employee not found"})
	}

	// curl -i -X DELETE http://localhost:9090/api/v1/emps/1
}

func OptionsEmployee(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Methods", "DELETE,POST, PUT")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	c.Next()
}