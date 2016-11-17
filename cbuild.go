//
//
//
package main

import (
    "fmt"
    "flag"
    "gopkg.in/yaml.v2"
    "io/ioutil"
    "path/filepath"
    "os/exec"
    "os"
    "strings"
    "unsafe"
)

//
// data structures
//
type Target struct {
    Name string
    Type string
}
type StringList struct {
    Type string
    Debug []string `yaml:",flow"`
    Release []string `yaml:",flow"`
    List []string `yaml:",flow"`
}
type Variable struct {
    Name string
    Value string
    Type string
    Build string
}
type Build struct {
    Name string
    Command string
    Files []string `yaml:",flow"`
}

type Data struct {
    Target Target
    Include []StringList `yaml:",flow"`
    Variable []Variable `yaml:",flow"`
    Define []StringList `yaml:",flow"`
    Option []StringList `yaml:",flow"`
    Archive_Option []StringList `yaml:",flow"`
    Prebuild []Build `yaml:",flow"`
    Postbuild []Build `yaml:",flow"`
    Source []StringList `yaml:",flow"`
    Subdir []StringList `yaml:",flow"`
}

type MyError struct {
    str string
}
func (m MyError) Error() string {
    return m.str
}

type BuildCommand struct {
    cmd string
    args string
    title string
}

type BuildResult struct {
    success bool
    create_list []string
}

type BuildInfo struct {
    variables map[string] string
    includes string
    defines string
    options string
    archive_options string
    subdir []string
    create_list []string
}

//
// global variables
//
var (
    isDebug bool
    isRelease bool
    target_type string
    outputdir string

    need_dir_list []string
    command_list []BuildCommand
)

//
// build functions
//
func getList(block []StringList) []string {
    lists := [] string{}
    for _,i := range block {
        if i.Type == "" || i.Type == target_type {
            for _,l := range i.List {
                lists = append(lists,l)
            }
            if isDebug == true {
                for _,d := range i.Debug {
                    lists = append(lists,d)
                }
            } else {
                for _,r := range i.Release {
                    lists = append(lists,r)
                }
            }
        }
    }
    return lists
}

func build(info BuildInfo,pathname string) (result BuildResult,err error) {
    loaddir := pathname
    if loaddir == "" {
        loaddir = "./"
    } else {
        loaddir += "/"
    }
    my_yaml := loaddir+"make.yml"
    buf, err := ioutil.ReadFile(my_yaml)
    if err != nil {
        e := MyError{ str : my_yaml + ": " + err.Error() }
        result.success = false
        return result,e
    }

    var d Data
    err = yaml.Unmarshal(buf, &d)
    if err != nil {
        e := MyError { str : my_yaml + ": " + err.Error() }
        result.success = false
        return result,e
    }

    NowTarget := d.Target
    if NowTarget.Name == "" {
        e := MyError{ str : "No Target" }
        result.success = false
        return result,e
    }

    for _,v := range d.Variable {
        if v.Type == "" || v.Type == target_type {
            info.variables[v.Name] = v.Value
        }
    }
    for _,i := range getList(d.Include) {
        abs, err := filepath.Abs(i)
        if err != nil {
            result.success = false
            return result,err
        }
        if target_type == "WIN32" {
            info.defines += " /I:" + abs
        } else {
            info.defines += " -I" + abs
        }
    }
    for _,d := range getList(d.Define) {
        if target_type == "WIN32" {
            info.defines += " /D:" + d
        } else {
            info.defines += " -D" + d
        }
    }
    for _,o := range getList(d.Option) {
        if target_type == "WIN32" {
            info.options += " /" + o
        } else {
            info.options += " -" + o
        }
    }
    for _,a := range getList(d.Archive_Option) {
        if target_type == "WIN32" {
            info.archive_options += "/" + a + " "
        } else {
            info.archive_options += "-" + a + " "
        }
    }

    files := getList(d.Source)

    subdirs := getList(d.Subdir)
    for _,s := range subdirs {
        sd := loaddir+s
        var r,e = build(info,sd)
        if r.success == false {
            return r,e
        }
        info.create_list = append(info.create_list,r.create_list...)
    }

    compiler := info.variables["compiler"]

    arg1 := info.includes + info.defines + info.options
    odir := outputdir + "/" + loaddir
    need_dir_list = append(need_dir_list,filepath.Clean(odir))
    create_list := []string{}
    for _,f := range files {
        sname := filepath.ToSlash(filepath.Clean(loaddir+f))
        oname := filepath.ToSlash(filepath.Clean(odir+f+".o"))
        create_list = append(create_list,oname)

        t := fmt.Sprintf("Compile: %s",sname)
        cmd := BuildCommand{
            cmd : compiler,
            args : arg1+" -o "+oname+" "+sname,
            title : t }
        command_list = append(command_list,cmd)
    }

    if NowTarget.Type == "library" {
        //
        // archive objects
        //
        if len(create_list) > 0 {
            arname := odir
            if target_type == "WIN32" {
                arname += NowTarget.Name + ".lib"
            } else {
                arname += "lib" + NowTarget.Name + ".a"
            }
            arname = filepath.ToSlash(filepath.Clean(arname))

            archiver := info.variables["archiver"]
            alist := ""
            for _,l := range create_list {
                alist += " " + l
            }

            t := fmt.Sprintf("Library: %s",arname)
            cmd := BuildCommand{
                cmd : archiver,
                args : info.archive_options+arname+alist,
                title : t }
            command_list = append(command_list,cmd)
            result.create_list = append(info.create_list,arname)
            fmt.Println(info.archive_options+arname+alist)
        }
    } else if NowTarget.Type == "execute" {
        //
        // link program
        //
        create_list = append(info.create_list,create_list...)
        if len(create_list) > 0 {
            trname := odir
            if target_type == "WIN32" {
                trname += NowTarget.Name + ".exe"
            } else {
                trname += NowTarget.Name
            }
            trname = filepath.ToSlash(filepath.Clean(trname))

            linker := info.variables["linker"]

            flist := ""
            for _,l := range create_list {
                flist += " " + l
            }

            t := fmt.Sprintf("Linking: %s",trname)
            cmd := BuildCommand{
                cmd : linker,
                args : "-o " + trname + flist,
                title : t }
            command_list = append(command_list,cmd)
            fmt.Println("-o " + NowTarget.Name + flist)
        }
    } else {
        //
        // othre...
        //
        result.create_list = append(info.create_list,create_list...)
    }
    result.success = true
    return result,nil
}

func main() {

    flag.BoolVar(&isRelease,"release",false,"release build")
    flag.BoolVar(&isDebug,"debug",true,"debug build")
    flag.StringVar(&target_type,"type","default","build target type")
    flag.StringVar(&outputdir,"o","build","build directory")
    flag.Parse()

    outputdir += "/" + target_type + "/"
    if isRelease {
        isDebug = false
        outputdir += "Release"
    } else {
        outputdir += "Debug"
    }

    build_info := BuildInfo{ variables : map[string] string{}, includes : "", defines : "" }
    var r,err = build(build_info,"")
    if r.success == false {
        fmt.Println("Error:",err.Error())
        os.Exit(1)
    }

    // setup directories
    for _,nd := range need_dir_list {
        os.MkdirAll(nd,os.ModePerm)
    }

    // execute build
    nlen := len(command_list)
    if nlen > 0 {
        for i,bs := range command_list {
            t := fmt.Sprintf("[%d/%d] %s",i+1,nlen,bs.title)
            fmt.Println(t)
            arg_list := strings.Split(bs.args," ")
            c,_ := exec.Command(bs.cmd,arg_list[0:]...).CombinedOutput()
            msg := *(*string)(unsafe.Pointer(&c))
            if msg != "" {
                fmt.Println(msg)
            }
        }
    }
}
//
//
