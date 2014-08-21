package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"os"
	"os/exec"

	"github.com/HouzuoGuo/tiedot/db"
	"github.com/codegangsta/cli"
	uuidgen "github.com/nu7hatch/gouuid"
)

type Pomodoro struct {
	conn        *db.DB
	timer       time.Duration
	shortBreak  time.Duration
	longBreak   time.Duration
	totalRounds int
	current     int
	currentTask *Task
}

type Task struct {
	docId       int
	uuid        string
	description string
	tags        []string
}

func checkError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Fatal error: %s", err.Error())
		os.Exit(1)
	}
}

func main() {
	dbDir := os.Getenv("HOME") + "/.heirloom"

	dbExist := true
	if _, err := os.Stat(dbDir); os.IsNotExist(err) {
		fmt.Println("Database directory does not exist yet, creating: %s", dbDir)
		dbExist = false
	}

	conn, err := db.OpenDB(dbDir)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	pomodoro := Pomodoro{
		conn:        conn,
		timer:       time.Minute * 25,
		shortBreak:  time.Minute * 5,
		longBreak:   time.Minute * 30,
		totalRounds: 5,
		current:     1,
		currentTask: nil,
	}

	if !dbExist {
		pomodoro.initDB()
	}

	app := cli.NewApp()
	app.Name = "hrl"
	app.Usage = "Simple command line tool for the Pomodoro® technique"
	app.Version = "0.0.1"
	app.Commands = []cli.Command{
		{
			Name:  "start",
			Usage: "Start a pomodoro timer",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "timer, t",
					Value: "25",
					Usage: "time for a single pomodoro round",
				},
				cli.StringFlag{
					Name:  "short, s",
					Value: "5",
					Usage: "time for a short break",
				},
				cli.StringFlag{
					Name:  "long, l",
					Value: "30",
					Usage: "time for a long break",
				},
				cli.StringFlag{
					Name:  "rounds, r",
					Value: "5",
					Usage: "number of pomodoro rounds",
				},
			},
			Action: func(c *cli.Context) {
				if c.String("timer") != "" {
					timer, err := strconv.Atoi(c.String("timer"))
					checkError(err)
					pomodoro.SetTimer(timer)
				}

				if c.String("short") != "" {
					short, err := strconv.Atoi(c.String("short"))
					checkError(err)
					pomodoro.SetShortBreak(short)
				}

				if c.String("long") != "" {
					long, err := strconv.Atoi(c.String("long"))
					checkError(err)
					pomodoro.SetLongBreak(long)
				}

				if c.String("rounds") != "" {
					rounds, err := strconv.Atoi(c.String("rounds"))
					checkError(err)
					pomodoro.SetRounds(rounds)
				}

				round := make(chan bool)
				smallBreak := make(chan bool)
				longBreak := make(chan bool)
				done := make(chan bool)

				go pomodoro.beginRound(round)
				go pomodoro.service(round, smallBreak, longBreak, done)

				<-done

				pomodoro.closeDB()
			},
		},
		{
			Name:  "add",
			Usage: "Add a task to the collection of tasks to accomplish",
			Action: func(c *cli.Context) {
				desc := c.Args().First()
				pomodoro.insertTask(desc, nil)
				pomodoro.closeDB()
			},
		},
	}
	app.Run(os.Args)
}

func (p *Pomodoro) beginRound(chanPomodoro chan bool) {
	p.pickTask()
	if p.currentTask == nil {
		fmt.Println("No more tasks in collection: You are on your own!")
	} else {
		fmt.Println("Current Task: " + p.currentTask.description)
	}
	p.tellBeginRound()
	time.Sleep(p.timer)
	p.tellEndRound()
	chanPomodoro <- true
}

func (p *Pomodoro) beginBreak(chanBreak chan bool) {
	tellBeginSmallBreak()
	time.Sleep(p.shortBreak)
	tellEndSmallBreak()
	chanBreak <- true
}

func (p *Pomodoro) beginLongBreak(chanLongBreak chan bool) {
	tellBeginLongBreak()
	time.Sleep(p.longBreak)
	tellEndLongBreak()
	chanLongBreak <- true
}

func (p *Pomodoro) service(chanPomodoro, chanBreak, chanLongBreak, chanDone chan bool) {
	fmt.Println("Pomodoro service started\n")
	for {
		select {

		case round := <-chanPomodoro:
			_ = round
			if p.current >= p.totalRounds {
				go p.beginLongBreak(chanLongBreak)
				p.current = 1
			} else {
				p.current += 1
				go p.beginBreak(chanBreak)
			}

		case smallBreak := <-chanBreak:
			_ = smallBreak
			go p.beginRound(chanPomodoro)

		case longBreak := <-chanLongBreak:
			_ = longBreak
			input := askAnotherSession()
			for input != "Y" && input != "N" {
				input = askAnotherSession()
			}
			if input == "Y" && p.currentTask != nil {
				go p.beginRound(chanPomodoro)
			} else {
				chanDone <- true
			}

		}
	}
}

func (p *Pomodoro) tellBeginRound() {
	toSay := "Pomodoro round" + strconv.Itoa(p.current) + "begins"
	exec.Command("say", toSay).Output()
}

func (p *Pomodoro) tellEndRound() {
	exec.Command("say", "Round ended").Output()

	input := askFinished()
	for input != "Y" && input != "N" {
		input = askFinished()
	}

	if input == "Y" && p.currentTask != nil {
		p.deleteTask(p.currentTask.docId)
		p.currentTask = nil
	}
}

func tellBeginSmallBreak() {
	exec.Command("say", "Have a small break!").Output()
}

func tellEndSmallBreak() {
	exec.Command("say", "This is the end of the small break. Let's get back to work!").Output()
}

func tellBeginLongBreak() {
	exec.Command("say", "Have a long break! You deserved it!").Output()
}

func tellEndLongBreak() {
	exec.Command("say", "This is the end of the long break. Let's get back to work!").Output()
}

func askFinished() string {
	fmt.Println("Have you finished the current task? (Y/N)")
	var input string
	fmt.Scanln(&input)
	return input
}

func askAnotherSession() string {
	fmt.Println("Ready for another pomodoro session? (Y/N)")
	var input string
	fmt.Scanln(&input)
	return input
}

func (p *Pomodoro) insertTask(description string, tags []string) error {
	tasks := p.conn.Use("Tasks")

	uuid, err := uuidgen.NewV4()
	checkError(err)

	docId, err := tasks.Insert(map[string]interface{}{
		"uuid": uuid.String(),
		"desc": description,
		"tags": []interface{}{}})

	_ = docId

	if err != nil {
		panic(err)
	}

	return nil
}

func (p *Pomodoro) pickTask() {
	tasks := p.conn.Use("Tasks")

	tasks.ForEachDoc(func(id int, docContent []byte) (willMoveOn bool) {
		var doc map[string]interface{}
		if json.Unmarshal(docContent, &doc) != nil {
			panic("cannot deserialize")
		}
		uuid := doc["uuid"].(string)
		desc := doc["desc"].(string)

		p.currentTask = &Task{docId: id, uuid: uuid, description: desc}

		return false
	})
}

func (p *Pomodoro) deleteTask(docId int) error {
	tasks := p.conn.Use("Tasks")
	if err := tasks.Delete(docId); err != nil {
		panic(err)
	}
	return nil
}

func (p *Pomodoro) initDB() {
	if err := p.conn.Create("Tasks"); err != nil {
		panic(err)
	}

	tasks := p.conn.Use("Tasks")
	tasks.Index([]string{"uuid"})
	tasks.Index([]string{"desc"})
}

func (p *Pomodoro) closeDB() {
	if err := p.conn.Close(); err != nil {
		panic(err)
	}
}

func (p *Pomodoro) syncDB() {
	if err := p.conn.Sync(); err != nil {
		panic(err)
	}
}

func (p *Pomodoro) SetTimer(timer int) {
	p.timer = time.Minute * time.Duration(timer)
}

func (p *Pomodoro) SetShortBreak(short int) {
	p.shortBreak = time.Minute * time.Duration(short)
}

func (p *Pomodoro) SetLongBreak(long int) {
	p.longBreak = time.Minute * time.Duration(long)
}

func (p *Pomodoro) SetRounds(rounds int) {
	p.totalRounds = rounds
}
