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
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

// EXAMPLES:
// dotfiles init git@bitbucket.org:andoco/dotfiles-core.git
// dotfiles pull
// dotfiles add dotfiles-core newfile.conf
// dotfiles push
// dotfiles add dotfiles-core wrongfile.conf
// dotfiles undo

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

	add     = app.Command("add", "Add a new or updated file to the repo staging index.")
	addRepo = add.Arg("repo-name", "Name of dotfile repo to stage to.").Required().String()
	addFile = add.Arg("file", "Path of a file to add to the dotfile repo.").Required().ExistingFile()

	push     = app.Command("push", "Push staged changes to the remote dotfile repo.")
	pushRepo = push.Arg("repo-name", "Name of dotfile repo to push changes to.").Required().String()

	undo     = app.Command("undo", "Undo staged changes for a dotfile repo.")
	undoRepo = undo.Arg("repo-name", "Name of dotfile repo to undo changes for.").Required().String()

	list        = app.Command("list", "List the dotfile repos in use.")
	listVerbose = list.Flag("verbose", "List all repo information").Bool()

	status     = app.Command("status", "Show the status of files for the dotfile repo.")
	statusRepo = status.Arg("repo-name", "Name of dotfile repo to show status for.").Required().String()
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
	}

	if cmdErr != nil {
		fmt.Println(cmdErr)
		os.Exit(1)
	}

	os.Exit(0)

	/*
		// setup transport.AuthMethod
		sshAuth, err := ssh.NewSSHAgentAuth("git")
		if err != nil {
			panic(err)
		}

		// clone bare dotfiles-core
		bareRepo, err := git.PlainClone("./dotfiles/dotfiles-core.git", true, &git.CloneOptions{
			URL:      "git@bitbucket.org:andoco/dotfiles-core.git",
			Progress: os.Stdout,
			Auth:     sshAuth,
		})
		if err != nil {
			panic(err)
		}

		// get work tree
		localFs := osfs.New("./worktree")
		localRepo, err := git.Open(bareRepo.Storer, localFs)
		if err != nil {
			panic(err)
		}

		wt, err := localRepo.Worktree()
		if err != nil {
			panic(err)
		}

		// checkout dotfiles-core master
		if err := wt.Checkout(&git.CheckoutOptions{}); err != nil {
			panic(err)
		}

		// modify file
		// TODO

		// show status
		status, err := wt.Status()
		if err != nil {
			panic(err)
		}
		fmt.Println(status)

		// commit to dotfiles-core master
		// TODO

		// push to origin
		// TODO
	*/
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

	auth, err := auth()
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
	err = wt.Checkout(&git.CheckoutOptions{})
	if err != nil {
		return
	}

	return
}

func executeStatus(repoName string) (err error) {
	workingRepo, err := openWorkingRepo(repoName)
	if err != nil {
		return
	}

	wt, err := workingRepo.Worktree()
	if err != nil {
		return
	}

	// show status
	status, err := wt.Status()
	if err != nil {
		panic(err)
	}
	fmt.Println(status)

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

func auth() (transport.AuthMethod, error) {
	sshAuth, err := ssh.NewSSHAgentAuth("git")
	if err != nil {
		return nil, err
	}
	return sshAuth, nil
}

func baseName(repoUrl string) (base string, err error) {
	base = filepath.Base(repoUrl)
	return
}

func openWorkingRepo(repoName string) (workingRepo *git.Repository, err error) {
	repoPath := filepath.Join(dotfilesBasedir, fmt.Sprintf("%s.git", repoName))
	fmt.Printf("repopath = %s\n", repoPath)
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
