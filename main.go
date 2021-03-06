package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/src-d/go-billy.v2/osfs"
	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

var (
	dotfilesBasedir = "./dotfiles"
	dotfilesWorkdir = "./workdir"
)

var (
	app = kingpin.New("dotfiles", "A program for working with dotfile git repos.")

	initCmd  = app.Command("init", "Use a new dotfiles repo.")
	initRepo = initCmd.Arg("repo-url", "URL of dotfile repo to use.").Required().String()

	pull     = app.Command("pull", "Pull changes from the remote dotfile repo.")
	pullRepo = pull.Arg("repo-name", "Name of dotfile repo to pull from.").Required().String()

	add     = app.Command("add", "Add a file to the repo staging index.")
	addRepo = add.Arg("repo-name", "Name of dotfile repo to stage to.").Required().String()
	addFile = add.Arg("file", "Path of a file to add to the dotfile repo.").Required().ExistingFile()

	save     = app.Command("save", "Save all modified and added files by committing and pushing to the remote dotfile repo.")
	saveRepo = save.Arg("repo-name", "Name of dotfile repo to save changes for.").Required().String()
	saveMsg  = save.Arg("msg", "Message describing the changes to the files.").Required().String()

	undo     = app.Command("undo", "Undo staged changes for a dotfile repo.")
	undoRepo = undo.Arg("repo-name", "Name of dotfile repo to undo changes for.").Required().String()

	list        = app.Command("list", "List the dotfile repos in use.")
	listVerbose = list.Flag("verbose", "List all repo information").Bool()

	status     = app.Command("status", "Show the status of files for the dotfile repo.")
	statusRepo = status.Arg("repo-name", "Name of dotfile repo to show status for.").String()
)

func main() {
	var cmdErr error
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case initCmd.FullCommand():
		cmdErr = executeInit(*initRepo)
	case list.FullCommand():
		cmdErr = executeList(*listVerbose)
	case status.FullCommand():
		cmdErr = executeStatus(*statusRepo)
	case add.FullCommand():
		cmdErr = executeAdd(*addRepo, *addFile)
	case save.FullCommand():
		cmdErr = executeSave(*saveRepo, *saveMsg)
	case pull.FullCommand():
		cmdErr = executePull(*pullRepo)
	}

	if cmdErr != nil {
		fmt.Println(cmdErr)
		os.Exit(1)
	}

	os.Exit(0)
}

func executeInit(repoUrl string) (err error) {
	fmt.Printf("Initialising repo %s\n", repoUrl)

	baseName, err := baseName(repoUrl)
	if err != nil {
		return
	}
	fmt.Printf("Repo basename = %s\n", baseName)
	basePath := filepath.Join(dotfilesBasedir, baseName)
	fmt.Printf("Repo basepath = %s\n", basePath)

	auth, err := getAuthMethod()
	if err != nil {
		return
	}

	// clone bare dotfiles-core
	bareRepo, err := git.PlainClone(basePath, true, &git.CloneOptions{
		URL:      repoUrl,
		Progress: os.Stdout,
		Auth:     auth,
	})
	if err != nil {
		return
	}

	// get work tree
	fmt.Printf("Workdir = %s\n", dotfilesWorkdir)
	localFs := osfs.New(dotfilesWorkdir)
	localRepo, err := git.Open(bareRepo.Storer, localFs)
	if err != nil {
		return
	}

	wt, err := localRepo.Worktree()
	if err != nil {
		return
	}

	// checkout dotfiles-core master
	err = wt.Checkout(&git.CheckoutOptions{Force: true})
	if err != nil {
		return
	}

	return
}

func executeStatus(repoName string) (err error) {
	repoNames := []string{}
	if repoName == "" {
		files, err := ioutil.ReadDir(dotfilesBasedir)
		if err != nil {
			return err
		}

		for _, f := range files {
			name := strings.TrimSuffix(f.Name(), ".git")
			repoNames = append(repoNames, name)
		}
	} else {
		repoNames = append(repoNames, repoName)
	}

	for _, repoName := range repoNames {
		fmt.Printf("%s:\n", repoName)

		workingRepo, err := openWorkingRepo(repoName)
		if err != nil {
			return err
		}

		wt, err := workingRepo.Worktree()
		if err != nil {
			return err
		}

		// show status
		status, err := wt.Status()
		if err != nil {
			return err
		}

		for f, s := range status {
			switch s.Worktree {
			case git.Modified:
				fallthrough
			case git.Added:
				fallthrough
			case git.Deleted:
				fmt.Printf("[%c] %s\n", s.Worktree, f)
			}
		}
	}

	return
}

func executeAdd(repoName string, addfile string) (err error) {
	fmt.Printf("Adding %s to %s\n", addfile, repoName)

	workingRepo, err := openWorkingRepo(repoName)
	if err != nil {
		return
	}

	wt, err := workingRepo.Worktree()
	if err != nil {
		return
	}

	absSrc, _ := filepath.Abs(addfile)
	absDst, _ := filepath.Abs(dotfilesWorkdir)
	path := strings.TrimPrefix(absSrc, absDst)
	path = strings.TrimPrefix(path, string(filepath.Separator))

	_, err = wt.Add(path)
	if err != nil {
		return
	}

	return
}

func executeList(verbose bool) (err error) {
	// TODO: verbose listing
	if verbose {
		return fmt.Errorf("verbose listing not implemented.")
	}

	files, err := ioutil.ReadDir(dotfilesBasedir)
	if err != nil {
		return
	}

	for _, f := range files {
		name := f.Name()
		base := strings.TrimSuffix(name, filepath.Ext(name))
		fmt.Println(base)
	}

	return
}

func executePull(repoName string) (err error) {
	workingRepo, err := openWorkingRepo(repoName)
	if err != nil {
		return
	}

	auth, err := getAuthMethod()
	if err != nil {
		return
	}

	err = workingRepo.Pull(&git.PullOptions{
		Auth: auth,
		//RemoteName:    "origin",
		//ReferenceName: "master",
		//Progress: os.Stdout,
	})
	if err != nil {
		return

	}

	return
}

func executeSave(repoName string, msg string) (err error) {
	workingRepo, err := openWorkingRepo(repoName)
	if err != nil {
		return
	}

	wt, err := workingRepo.Worktree()
	if err != nil {
		return
	}

	author, err := getAuthor()
	if err != nil {
		return
	}

	// commit changed or staged files
	_, err = wt.Commit(msg, &git.CommitOptions{
		All:    true,
		Author: &author,
	})
	if err != nil {
		return
	}

	auth, err := getAuthMethod()
	if err != nil {
		return
	}

	err = workingRepo.Push(&git.PushOptions{
		//RemoteName: "origin",
		/*RefSpecs: []config.RefSpec{
			"master",
		},*/
		Auth: auth,
	})
	if err != nil {
		return
	}

	return
}

func getAuthMethod() (transport.AuthMethod, error) {
	sshAuth, err := ssh.NewSSHAgentAuth("git")
	if err != nil {
		return nil, err
	}
	return sshAuth, nil
}

func getAuthor() (sig object.Signature, err error) {
	sig = object.Signature{
		Name:  os.Getenv("GIT_AUTHOR_NAME"),
		Email: os.Getenv("GIT_AUTHOR_EMAIL"),
	}

	if sig.Name == "" || sig.Email == "" {
		err = fmt.Errorf("GIT_AUTHOR_NAME and GIT_AUTHOR_EMAIL envionment variables must exist.")
	}

	return
}

func baseName(repoUrl string) (base string, err error) {
	base = filepath.Base(repoUrl)
	return
}

func openWorkingRepo(repoName string) (workingRepo *git.Repository, err error) {
	repoPath := filepath.Join(dotfilesBasedir, fmt.Sprintf("%s.git", repoName))
	repoStorer, err := filesystem.NewStorage(osfs.New(repoPath))
	if err != nil {
		return
	}

	workingFs := osfs.New(dotfilesWorkdir)
	workingRepo, err = git.Open(repoStorer, workingFs)
	if err != nil {
		return
	}

	return
}
