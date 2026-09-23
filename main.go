package main

import (
	"context"
	"embed"
	"gl3/cli"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

//go:embed builtins/*.gl3 builtins/*.ll
var builtinFs embed.FS

func main() {
	rootCmd := &cobra.Command{
		Use:   "gl3",
		Short: "gl3 main command",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("You ran the root command. Try `gl3 build x.gl3`")
		},
	}

	var buildOpts cli.BuildOpts
	buildCmd := &cobra.Command{
		Use:   "build",
		Short: "Compile gl3 files to an executable",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := cli.RunBuildCmd(builtinFs, args, &buildOpts)
			return err
		},
	}
	buildCmd.Flags().BoolVar(&buildOpts.Dbg, "dbg", false, "Prints out the AST for all compiled files, along with the `clang` command used for compilation")
	buildCmd.Flags().BoolVar(&buildOpts.Shared, "shared", false, "Compiles the code as a shared library")
	buildCmd.Flags().StringVarP(&buildOpts.Output, "output", "o", "./out", "Changes the name of the output executable")
	buildCmd.Flags().StringVar(&buildOpts.Emitter, "emitter", "llvm", "Code emitter (available: llvm)")
	buildCmd.Flags().BoolVar(&buildOpts.O1, "O1", false, "Compiles the code with optimization level 1")
	buildCmd.Flags().BoolVar(&buildOpts.O2, "O2", false, "Compiles the code with optimization level 1")
	buildCmd.Flags().BoolVar(&buildOpts.O3, "O3", false, "Compiles the code with optimization level 1")

	var exDefOpts cli.ExDefOpts
	exDefCmd := &cobra.Command{
		Use:   "exdef",
		Short: "Extracts define statements from a C header file and outputs a .gl3 file defining them as constants",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.RunExDef(&exDefOpts)
		},
	}
	exDefCmd.Flags().StringVarP(&exDefOpts.InFile, "input", "i", "", "Path to the input header")
	exDefCmd.Flags().StringVarP(&exDefOpts.OutFile, "output", "o", "", "Path to the output .gl3 file")
	exDefCmd.MarkFlagRequired("input")
	exDefCmd.MarkFlagRequired("output")

	rootCmd.AddCommand(buildCmd)
	rootCmd.AddCommand(exDefCmd)

	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithNotifySignal(os.Interrupt, os.Kill),
	); err != nil {
		os.Exit(1)
	}
}
