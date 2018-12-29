package main

import(
	"os"
	"fmt"
	"errors"
	"path/filepath"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

// Global variables
var HOMEPATH   string
var REPOPATH   string
var SYNCPATH   string
var DIFFPATH   string
var REPONAME   string
var AWSPROFILE string = "default"
var AWSREGION  string = "us-east-1"
var DYNAMODB   *dynamodb.DynamoDB

// TODO: Let function receive argument and return a more detailed help
func showHelp() {
	fmt.Println("")
	fmt.Println("Usage: dynasync [option] [command] file ...")
	fmt.Println("")
	fmt.Println("Available options are:")
	fmt.Println(" --aws-profile:	use the given aws profile," +
		" defaults to 'default'")
	fmt.Println(" --aws-region:	use the given aws region," +
		" defaults to 'us-east-1'")
	fmt.Println("")
	fmt.Println("Available command are:")
	fmt.Println(" init:		create configuration file")
	fmt.Println(" commit:	send modifications to remote table")
	fmt.Println(" tag:		adds a tag to current repo status")
	fmt.Println(" clone:		initiates a copy of a remote repo")
	fmt.Println("")
	fmt.Println("Optional parameters for init:")
	fmt.Println(" name:		set the repo name for name")
	fmt.Println("")
	fmt.Println("Needed parameters for commit:")
	fmt.Println(" files:		list of files to be sent to repository")
	fmt.Println(" -m message:	commit message")
	fmt.Println("")
	fmt.Println("Needed parameters for tag:")
	fmt.Println(" tag:		tag name")
	fmt.Println("")
	fmt.Println("Needed parameters for clone:")
	fmt.Println(" repo:		repo name")
}

func loadGlobalConfig() (error) {
	configFile, err := os.Open(HOMEPATH + "/.config/sync/global.conf")
	if err != nil {
		return err
	}

	// TODO: return error if read failed
	fmt.Fscanf(configFile, "profile: %s\n", &AWSPROFILE)
	fmt.Fscanf(configFile, "region: %s\n", &AWSREGION)
	configFile.Close()
	return nil
}

func findConfig() (error) {
	// Try to find the config folder in parent folders
	for REPOPATH != "/" {
		_, err := os.Stat(REPOPATH + "/.sync/")
		if err == nil {
			SYNCPATH = REPOPATH + "/.sync/"
			DIFFPATH = SYNCPATH + "diff/"

			// Open config file and read config
			configFile, err := os.Open(SYNCPATH + "repo.conf")
			if err != nil {
				return err
			}

			fmt.Fscanf(configFile, "name: %s\n", &REPONAME)
			fmt.Fscanf(configFile, "profile: %s\n", &AWSPROFILE)
			fmt.Fscanf(configFile, "region: %s\n", &AWSREGION)
			configFile.Close()

			return nil
		}
		REPOPATH = filepath.Dir(REPOPATH)
	}

	return errors.New("config dir not found")
}

// FIXME: Let init again with another name, now it creates only the new
// remote table
func initRepo(repo string) {

	// Check args
	if repo != ""  {
		REPONAME = "repo-" + repo
	} else {
		REPONAME = "repo-" + filepath.Base(REPOPATH)
	}

	// Load global config if exists
	_, err := os.Stat(HOMEPATH + "/.config/sync/global.conf")
	if err != nil {
		os.MkdirAll(HOMEPATH + "/.config/sync", 0777)
		globalConfFile, err := os.Create(HOMEPATH + "/.config/sync/global.conf")
		_, err = globalConfFile.Write([]byte(
			"profile: " + AWSPROFILE + "\n" +
			"region: " + AWSREGION + "\n"))
			globalConfFile.Close()
		if err != nil {
			panic("error writing global config file: " + err.Error())
		}
	} else {
		loadGlobalConfig()
	}

	// Local repo config
	os.MkdirAll(SYNCPATH, 0777)
	os.RemoveAll(DIFFPATH)
	os.MkdirAll(DIFFPATH, 0777)

	// Create .sync/repo.conf file
	_, err = os.Stat(SYNCPATH + "repo.conf")

	if err == nil {
		fmt.Println("config file already exists")
		findConfig()
	} else {
		configFile, err := os.Create(SYNCPATH + "repo.conf")

		_, err = configFile.Write([]byte(
			"name: " + REPONAME + "\n" +
			"profile: " + AWSPROFILE + "\n" +
			"region: " + AWSREGION + "\n"))
		if err != nil {
			panic("error writing to config file: " + err.Error())
		}
	}

	hasRepo, err := checkRepoExistence(REPONAME)
	if err != nil {
		panic("error checking for repo existence: " + err.Error())
	} else if !hasRepo {
		err = createRepo()
		if err != nil {
			panic("error creating remote repo: " + err.Error())
		}
		fmt.Println("remote table " + REPONAME + " created")
	} else {
		fmt.Println("remote repo found")
	}

	fmt.Println("done")
}

func main() {
	// Keep track of the repo path
	HOMEPATH = os.Getenv("HOME")
	_ = loadGlobalConfig()
	REPOPATH, _ = os.Getwd()
	SYNCPATH = REPOPATH + "/.sync/"
	DIFFPATH = SYNCPATH + "diff/"

	if len(os.Args) == 1 {
		fmt.Println("Dynasync v1.0.0: A very simple version control system")
		showHelp()
		return
	}

	// TODO: Process all arguments before taking an action
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "--aws-profile":
			if i + 2 > len(os.Args) {
				fmt.Println("error: missing argument")
				showHelp()
				return
			}
			AWSPROFILE = os.Args[i + 1]
			i++
			break
		case "--aws-region":
			if i + 2 > len(os.Args) {
				fmt.Println("error: missing argument")
				showHelp()
				return
			}
			AWSREGION = os.Args[i + 1]
			i++
			break
		case "--help":
			fallthrough
		case "help":
			fallthrough
		case "-h":
			fmt.Println("Dynasync v1.0.0: A very simple version control system")
			showHelp()
			return
		case "init":
			DYNAMODB = startDynamoDBSession()
			if i + 2 > len(os.Args) {
				initRepo("")
			} else {
				initRepo(os.Args[i + 1])
			}
			return
		case "commit":
			if findConfig() != nil {
				fmt.Println("error: Config file not found")
				return
			}

			DYNAMODB = startDynamoDBSession()
			commit(os.Args[i + 1:])
			return
		case "tag":
			if i + 2 > len(os.Args) {
				fmt.Println("error: missing argument")
				showHelp()
				return
			}
			if findConfig() != nil {
				fmt.Println("error: Config file not found")
				return
			}

			DYNAMODB = startDynamoDBSession()
			err := tag(os.Args[i + 1])
			if err != nil {
				fmt.Println("error taging: " + err.Error())
			}
			return
		case "clone":
			if i + 2 > len(os.Args) {
				fmt.Println("error: missing argument")
				showHelp()
				return
			}

			DYNAMODB = startDynamoDBSession()
			clone(os.Args[i + 1])
			return
		default:
			fmt.Println("error: illegal option", os.Args[i])
			showHelp()
			return
		}
	}
}

