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
	round       time.Duration
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
	// TODO Put into HOME directory in .heirloom
	dbDir := "/tmp/heirloom"
	// os.RemoveAll(dbDir)
	// defer os.RemoveAll(dbDir)

	conn, err := db.OpenDB(dbDir)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	pomodoro := Pomodoro{
		conn:        conn,
		round:       time.Minute * 20,
		shortBreak:  time.Minute * 5,
		longBreak:   time.Minute * 20,
		totalRounds: 5,
		current:     1,
		currentTask: nil,
	}

	pomodoro.initDB()

	app := cli.NewApp()
	app.Name = "hrl"
	app.Usage = "Simple command line tool for the Pomodoro® technique"
	app.Version = "0.0.1"
	app.Flags = []cli.Flag{
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
			Value: "20",
			Usage: "time for a long break",
		},
		cli.StringFlag{
			Name:  "rounds, r",
			Value: "1",
			Usage: "number of pomodoro rounds",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "start",
			ShortName: "s",
			Usage:     "Start a pomodoro timer",
			Action: func(c *cli.Context) {
				// TODO handle cli flags

				round := make(chan bool)
				smallBreak := make(chan bool)
				longBreak := make(chan bool)
				done := make(chan bool)

				go pomodoro.beginRound(round)
				go pomodoro.service(round, smallBreak, longBreak, done)

				<-done
			},
		},
		{
			Name:      "add",
			ShortName: "a",
			Usage:     "Add a task to the collection of tasks to accomplish",
			Action: func(c *cli.Context) {
				desc := c.Args().First()

				uuid, err := uuidgen.NewV4()
				checkError(err)

				pomodoro.insertTask(uuid.String(), desc, nil)
			},
		},
	}
	app.Run(os.Args)
}

func (p *Pomodoro) beginRound(chanPomodoro chan bool) {
	p.pickTask()
	if p.currentTask == nil {
		fmt.Println("No more tasks in collection: You are on your own! Or stop the round and add a new task with `hrl add`")
	} else {
		fmt.Println("Current Task: " + p.currentTask.description)
	}
	p.tellBeginRound()
	time.Sleep(p.round)
	p.tellEndRound()
	chanPomodoro <- true
}

func (p *Pomodoro) beginBreak(chanBreak chan bool) {
	tellBeginSmallBreak()
	time.Sleep(p.shortBreak)
	chanBreak <- true
}

func (p *Pomodoro) beginLongBreak(chanLongBreak chan bool) {
	tellBeginLongBreak()
	time.Sleep(p.longBreak)
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
				// TODO Ask if continue?
				chanDone <- true
			} else {
				p.current += 1
				go p.beginBreak(chanBreak)
			}

		case smallBreak := <-chanBreak:
			_ = smallBreak
			tellEndSmallBreak()
			go p.beginRound(chanPomodoro)

		case longBreak := <-chanLongBreak:
			_ = longBreak
			tellEndLongBreak()
			go p.beginRound(chanPomodoro)

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
		p.pickTask()
	} else {
		// TODO ask if want to continue with this task or pick another one
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
	fmt.Printf("Have you finished the current task? (Y/N)")
	var input string
	fmt.Scanln(&input)
	return input
}

func (p *Pomodoro) insertTask(uuid string, description string, tags []string) error {
	tasks := p.conn.Use("Tasks")

	_, err := tasks.Insert(map[string]interface{}{
		"uuid": uuid,
		"desc": description,
		"tags": []interface{}{}})

	if err != nil {
		panic(err)
	}

	return nil
}

func (p *Pomodoro) pickTask() {
	tasks := p.conn.Use("Tasks")

	tasks.ForEachDoc(func(id int, docContent []byte) (willMoveOn bool) {
		// Take first task encountered
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
	p.conn.Create("Tasks")

	tasks := p.conn.Use("Tasks")
	tasks.Index([]string{"uuid"})
	tasks.Index([]string{"desc"})

	p.syncDB()
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
