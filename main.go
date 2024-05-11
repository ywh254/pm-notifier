package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/ywh254/robot"
	"github.com/ywh254/robot/dingtalk"
)

const (
	DingTalk = iota
	WeChat
	Lark
)

const (
	NotSkipped = iota
	SkippedHoliday
)

var (
	configDir      = "config"
	configName     = "config.toml"
	memberFileName = "members.csv"
	bot            robot.Robot
	retryInterval  = 60 * time.Second
	replaceWord    = "{}"
	members        map[string]string
)

type TaskIteration struct {
	ID string `json:"_id"`
}

type Date struct {
	IsHoliday bool `json:"isHoliday"`
}

func main() {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	viper.SetConfigFile(filepath.Join(dir, configDir, configName))
	viper.SetConfigType("toml")
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("read config file error: %s", err))
	}

	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		panic(err)
	}

	fmt.Printf("config is: %+v\n", config)

	members, err = readMembers(filepath.Join(dir, configDir, memberFileName))
	if err != nil {
		panic(err)
	}

	fmt.Printf("members is: %+v\n", members)

	if err := startPeriod(config); err != nil {
		fmt.Printf("error is: %s", err)
	}
}

func startPeriod(c *Config) error {
	startTime := c.Notification.StartTime
	ts := strings.Split(startTime, ",")

	// setup robot
	switch c.Notification.Type {
	case DingTalk:
		bot = dingtalk.New([]string{c.Notification.Token}, c.Notification.Key)
	case WeChat:
		// TODO
	case Lark:
		// TODO
	}

	for _, t := range ts {
		now := time.Now()
		tmp, err := time.Parse("3:04PM", t)
		if err != nil {
			return err
		}
		tmp = time.Date(now.Year(), now.Month(), now.Day(), tmp.Hour(), tmp.Minute(), 0, 0, time.Local)

		var delay time.Duration
		if now.After(tmp) {
			delay = tmp.Add(24 * time.Hour).Sub(now)
		} else {
			delay = tmp.Sub(now)
		}
		fmt.Printf("delay: %s", delay)
		time.AfterFunc(delay, func() {
			// do notice
			go func() {
				if err := handle(c); err != nil {
					fmt.Printf("handle error: %s", err)
				}
			}()

			// tick every day
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					go func() {
						if err := handle(c); err != nil {
							fmt.Printf("handle error: %s", err)
						}
					}()
				}
			}
		})
	}

	select {}
}

func isDateSkipped(c *Config) bool {
	if c.Notification.ExcludeDate == NotSkipped {
		return false
	}

	if c.Notification.ExcludeDate == SkippedHoliday {
		// judge now if is holiday
		resp, err := http.Get(c.Notification.DateUrl)
		if err != nil {
			fmt.Printf("get date url error: %s\n", err)
			return false
		}
		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("read date url response error: %s\n", err)
			return false
		}

		fmt.Printf("get date: %s\n", data)

		var res Date
		if err := json.Unmarshal(data, &res); err != nil {
			return false
		}

		if res.IsHoliday {
			return true
		}
	}

	return false
}

func handle(c *Config) error {
	if isDateSkipped(c) {
		fmt.Printf("today is holiday, no notification")
		return nil
	}

	// first get iteration
	data, err := request(c, c.Project.TaskIterationListUrl)
	if err != nil {
		return err
	}

	its := make([]TaskIteration, 10)
	if err := json.Unmarshal(data, &its); err != nil {
		return err
	}

	if len(its) == 0 {
		return fmt.Errorf("task iteration is empty")
	}

	fmt.Printf("task iterations: %+v\n", its)

	taskListUrl := strings.Replace(c.Project.TaskListUrl, replaceWord, its[0].ID, 1)
	fmt.Printf("task list url: %s\n", taskListUrl)

	// read body and unmarshal to json
	data, err = request(c, taskListUrl)
	if err != nil {
		return err
	}

	res := make([]map[string]interface{}, c.Project.TaskListMaxSize)
	if err := json.Unmarshal(data, &res); err != nil {
		return err
	}

	nowTaskList, notDoneTaskList, noDeadlineTaskList, taskExecutorMap, err := handleResult(c, res)
	if err != nil {
		return err
	}

	fmt.Printf("now task list: %v\n\n, not done task list %v\n\n, no deadline task list %v\n\n, task executor list %v\n\n", nowTaskList, notDoneTaskList, noDeadlineTaskList, taskExecutorMap)
	if err := sendNotification(nowTaskList, notDoneTaskList, noDeadlineTaskList, taskExecutorMap, c); err != nil {
		return err
	}

	return nil
}

func request(c *Config, url string) ([]byte, error) {
	httpClient := http.DefaultClient
	// first get iteration
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if err := setAuth(c, req); err != nil {
		return nil, err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request the task iteration list url error, http status code: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func sendNotification(nowTaskList, notDoneTaskList, noDeadlineTaskList []map[string]string, taskExecutorMap map[string]int, c *Config) error {
	send := func() error {
		atPhoneList := getPhoneList(taskExecutorMap)

		return bot.SendMarkdownMessage(c.Notification.Title, fmt.Sprintf(`# %s
---
## %s
%s
## %s
%s
## %s
%s
---
`, c.Notification.Title, c.Notification.NowTaskListDesc, toMarkdown(nowTaskList), c.Notification.NotDoneTaskListDesc, toMarkdown(notDoneTaskList), c.Notification.NoDeadlineTaskListDesc, toMarkdown(noDeadlineTaskList)), dingtalk.WithAtMembers(atPhoneList))
	}

	for err := send(); err != nil; {
		fmt.Printf("send occurs error: %s\n", err)
		time.Sleep(retryInterval)
	}
	return nil
}

func toMarkdown(list []map[string]string) string {
	dst := "- 执行人 任务名称 截止时间\n"
	for _, l := range list {
		dst = fmt.Sprintf("%s - %s %s %s\n", dst, l["executor_name"], l["task_name"], l["deadline"])
	}
	return dst
}

func handleResult(c *Config, result []map[string]interface{}) ([]map[string]string, []map[string]string, []map[string]string, map[string]int, error) {
	notDoneTaskList := make([]map[string]string, 0)
	nowTaskList := make([]map[string]string, 0)
	noDeadlineTaskList := make([]map[string]string, 0)
	taskExecutorMap := make(map[string]int, 0)

	for _, r := range result {
		// judge deadline if is today
		deadline, err := getValue[string](c, r, c.Project.FieldMapping.TaskDeadline)
		if err != nil {
			// if not set deadline, add to noDeadlineTaskList
			task_name, err := getValue[string](c, r, c.Project.FieldMapping.TaskName)
			executor_name, err := getValue[string](c, r, c.Project.FieldMapping.ExecutorName)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			m := make(map[string]string)
			m["task_name"] = task_name
			m["executor_name"] = executor_name
			noDeadlineTaskList = append(noDeadlineTaskList, m)

			taskExecutorMap[executor_name] = 0
			continue
		}

		deadlineFormat := c.Project.TaskDeadlineFormat

		t, err := time.Parse(deadlineFormat, deadline)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		// judge the task if is done
		taskIsDone, err := getValue[bool](c, r, c.Project.FieldMapping.TaskDone)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		if !taskIsDone {
			task_name, err := getValue[string](c, r, c.Project.FieldMapping.TaskName)
			executor_name, err := getValue[string](c, r, c.Project.FieldMapping.ExecutorName)
			if err != nil {
				return nil, nil, nil, nil, err
			}

			m := make(map[string]string)
			m["task_name"] = task_name
			m["executor_name"] = executor_name
			m["deadline"] = fmt.Sprintf("%d-%d-%d", t.Year(), t.Month(), t.Day())

			today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
			if today.Before(t) && today.Add(24*time.Hour).After(t) {
				nowTaskList = append(nowTaskList, m)

				taskExecutorMap[executor_name] = 0
			}
			if today.After(t) {
				notDoneTaskList = append(notDoneTaskList, m)

				taskExecutorMap[executor_name] = 0
			}

		}
	}
	return nowTaskList, notDoneTaskList, noDeadlineTaskList, taskExecutorMap, nil
}

func getValue[T bool | int | string](c *Config, result map[string]interface{}, key string) (T, error) {
	keys := strings.Split(key, ".")

	var zero T
	if len(keys) == 1 {
		kind := reflect.ValueOf(result[keys[0]]).Kind()
		switch kind {
		case reflect.Bool, reflect.Int, reflect.String:
			return result[keys[0]].(T), nil
		default:
			return zero, fmt.Errorf("key: %s, type %T error, not bool, init or string", keys[0], kind)
		}
	}

	rv, ok := result[keys[0]].(map[string]interface{})
	if !ok {
		return zero, fmt.Errorf("key: %s result %T to map[string]interface{} failed", keys[0], result[keys[0]])
	}

	return getValue[T](c, rv, strings.Join(keys[1:], "."))
}

func getPhoneList(taskExecutorMap map[string]int) []string {
	var phoneList []string

	for executor, _ := range taskExecutorMap {
		e, ok := members[executor]
		if ok {
			phoneList = append(phoneList, e)
		} else {
			// TODO send notification
			fmt.Printf("warning: executor %s not in member list!", executor)
		}
	}
	return phoneList
}

func setAuth(c *Config, req *http.Request) error {
	switch c.Login.LoginType {
	case "cookie":
		req.Header.Set("Cookie", c.Login.Cookie)
	case "password":
		// use username/password login to get token
	case "token":
		// set token
	default:
		return errors.New("login type error")
	}

	return nil
}
