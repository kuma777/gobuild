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
    "os"
    "strings"
    //"os/exec"
    //"unsafe"
)

//
// data structures
//
type Target struct {
    Name string
    Type string
    By_Target string
}
type StringList struct {
    Type string
    Target string
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
    Target string
    Type string
    Source []StringList `yaml:",flow"`
}

type Data struct {
    Target []Target `yaml:",flow"`
    Include []StringList `yaml:",flow"`
    Variable []Variable `yaml:",flow"`
    Define []StringList `yaml:",flow"`
    Option []StringList `yaml:",flow"`
    Archive_Option []StringList `yaml:",flow"`
    Convert_Option []StringList `yaml:",flow"`
    Link_Option []StringList `yaml:",flow"`
    Prebuild []Build `yaml:",flow"`
    Postbuild []Build `yaml:",flow"`
    Source []StringList `yaml:",flow"`
    Convert_List []StringList `yaml:",flow"`
    Subdir []StringList `yaml:",flow"`
}

//
// error
//

type MyError struct {
    str string
}
func (m MyError) Error() string {
    return m.str
}

//
// build information
//

//
type BuildCommand struct {
    cmd string
    cmdtype string
    cmdalias string
    args []string
    infiles []string
    outfile string
    depfile string
    depends []string
    need_cmd_alias bool
}

//
type BuildResult struct {
    success bool
    create_list []string
}

//
type BuildInfo struct {
    variables map[string] string
    includes []string
    defines []string
    options []string
    archive_options []string
    convert_options []string
    link_options []string
    select_target string
    target string
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
    target_name string
    outputdir string
    outputdir_set bool
    append_rules map[string] string

    command_list []BuildCommand
)

//
//
// build functions
//
//

//
//
//
func getList(block []StringList,target_name string) []string {
    lists := [] string{}
    for _,i := range block {
        if (i.Type == "" || i.Type == target_type) && (i.Target == "" || i.Target == target_name) {
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

//
// archive objects
//
func create_archive(info BuildInfo,odir string,create_list []string,target_name string) string {

    arname := odir
    if target_type == "WIN32" {
        arname += target_name + ".lib"
    } else {
        arname += "lib" + target_name + ".a"
    }
    arname = filepath.ToSlash(filepath.Clean(arname))

    archiver := info.variables["archiver"]

    cmd := BuildCommand{
        cmd : archiver,
        cmdtype : "ar",
        args : info.archive_options,
        infiles : create_list,
        outfile : arname,
        need_cmd_alias : true }
    command_list = append(command_list,cmd)

    return arname
}

//
// link objects
//
func create_link(info BuildInfo,odir string,create_list []string,target_name string) {
    trname := odir
    if target_type == "WIN32" {
        trname += target_name + ".exe"
    } else {
        trname += target_name
    }
    trname = filepath.ToSlash(filepath.Clean(trname))

    linker := info.variables["linker"]

    cmd := BuildCommand{
        cmd : linker,
        cmdtype : "link",
        args : info.link_options,
        infiles : create_list,
        outfile : trname,
        need_cmd_alias : true }
    command_list = append(command_list,cmd)
    //fmt.Println("-o " + NowTarget.Name + flist)
}

//
// convert objects
//
func create_convert(info BuildInfo,loaddir string,odir string,create_list []string,target_name string) {
    cvname := odir + target_name
    cvname = filepath.ToSlash(filepath.Clean(cvname))
    converter := info.variables["converter"]

    clist := []string{}
    for _,f := range create_list {
        clist = append(clist,filepath.ToSlash(filepath.Clean(loaddir+f)))
    }

    cmd := BuildCommand{
        cmd : converter,
        cmdtype : "convert",
        args : info.convert_options,
        infiles : clist,
        outfile : cvname,
        need_cmd_alias : true }
    command_list = append(command_list,cmd)
}

//
// option
//
func append_option(lists []string,opt string,opt_pre string) []string {
    sl := strings.Split(opt," ")
    sl[0] = opt_pre+sl[0]
    for _,so := range sl {
        lists = append(lists,so)
    }
    return lists
}

//
// target
//
func get_target(info BuildInfo,tlist []Target) (Target,string,bool) {
    if info.select_target != "" {
        // search target
        for _,t := range tlist {
            if info.select_target == t.Name {
                return t,"_"+info.select_target,true
            }
        }

    } else {
        if info.target != "" {

            // search by_target
            for _,t := range tlist {
                if info.target == t.By_Target {
                    return t,"_"+info.target,true
                }
            }
            // search target
            for _,t := range tlist {
                if info.target == t.Name {
                    return t,"_"+info.target,true
                }
            }
        }
        if len(tlist) > 0 {
            t := tlist[0]
            if info.target == "" {
                return t,"_"+t.Name,true
            } else {
                return t,"",true
            }
        }
    }
    return Target{},"",false
}

//
// pre build
//
func create_prebuild(info BuildInfo,odir string,plist []Build) error {
    for _,p := range plist {
        if (p.Target == "" || p.Target == info.target) && (p.Type == "" || p.Type == target_type) {
            // regist prebuild
            srlist := getList(p.Source,info.target)
            if len(srlist) == 0 {
                e := MyError{ str : "build command: "+p.Name+" is empty source." }
                return e
            }
            ur,ok := info.variables[p.Command]
            if ok == false {
                e := MyError{ str : "build command: <"+p.Command+"> is not found.(use by "+p.Name+")"}
                return e
            }
            mycmd := strings.Replace(filepath.ToSlash(filepath.Clean(odir+p.Command)),"/","_",-1)
            deps := []string{}
            _,af := append_rules[mycmd];
            if af == false {
                if ur[0] == '$' {
                    url := strings.Split(ur," ")
                    ucmd := url[0]
                    ur = filepath.ToSlash(filepath.Clean(odir+ucmd[1:len(ucmd)]))
                    deps = append(deps,ur)
                    for i,uu := range url {
                        if i > 0 {
                            ur += " "+uu
                        }
                    }
                }
                append_rules[mycmd] = ur
            }

            cmd := BuildCommand{
                cmd : p.Command,
                cmdtype : mycmd,
                depends : deps,
                infiles : srlist,
                outfile : filepath.ToSlash(filepath.Clean(odir+p.Name)),
                need_cmd_alias : false }
            command_list = append(command_list,cmd)
        }
    }
    return nil
}



//
// build main
//
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
        e := MyError{ str : my_yaml + ": " + err.Error() }
        result.success = false
        return result,e
    }

    //
    // select target
    //
    NowTarget,objs_suffix,ok := get_target(info,d.Target)
    if ok == false {
        e := MyError{ str : "No Target" }
        result.success = false
        return result,e
    }
    if info.target == "" {
        info.target = NowTarget.Name
        fmt.Println("gobuild: make target: "+info.target)
    }
    info.select_target = ""

    opt_pre := info.variables["option_prefix"]
    //
    // get rules
    //
    for _,v := range d.Variable {
        if v.Type == "" || v.Type == target_type {
            info.variables[v.Name] = v.Value
        }
    }
    if outputdir_set == false {
        def_type, dok := info.variables["default_type"]
        if dok == true {
            target_type = def_type
        }
        outputdir += "/" + target_type + "/"
        if isRelease {
            outputdir += "Release"
        } else {
            outputdir += "Debug"
        }
        outputdir_set = true
    }

    for _,i := range getList(d.Include,info.target) {
        if filepath.IsAbs(i) == false {
            i = loaddir + i
        }
        abs, err := filepath.Abs(i)
        if err != nil {
            result.success = false
            return result,err
        }
        info.includes = append(info.includes,opt_pre + "I" + filepath.ToSlash(abs))
    }
    for _,d := range getList(d.Define,info.target) {
        info.defines = append(info.defines,opt_pre + "D" + d)
    }
    for _,o := range getList(d.Option,info.target) {
        info.options = append_option(info.options,o,opt_pre)
    }
    for _,a := range getList(d.Archive_Option,info.target) {
        info.archive_options = append_option(info.archive_options,a,"")
    }
    for _,c := range getList(d.Convert_Option,info.target) {
        info.convert_options = append_option(info.convert_options,c,"")
    }
    for _,l := range getList(d.Link_Option,info.target) {
        info.link_options = append_option(info.link_options,l,opt_pre)
    }

    files := getList(d.Source,info.target)
    cvfiles := getList(d.Convert_List,info.target)

    // sub-directories
    subdirs := getList(d.Subdir,info.target)
    for _,s := range subdirs {
        sd := loaddir+s
        var r,e = build(info,sd)
        if r.success == false {
            return r,e
        }
        info.create_list = append(info.create_list,r.create_list...)
    }

    compiler := info.variables["compiler"]

    odir := outputdir + "/" + loaddir
    objdir := outputdir + "/" + loaddir + ".objs"+objs_suffix+"/"
    create_list := []string{}

    // pre build files
    err = create_prebuild(info,odir,d.Prebuild)
    if err != nil {
        return result,err
    }

    if len(files) > 0 {
        arg1 := append(info.includes,info.defines...)

        my_list := make([]BuildCommand,len(files))
        for i,f := range files {
            of := f
            if f[0] == '$' {
                of = f[1:len(f)]
                f = odir + of
            }
            sname := filepath.ToSlash(filepath.Clean(loaddir+f))
            oname := filepath.ToSlash(filepath.Clean(objdir+of+".o"))
            dname := filepath.ToSlash(filepath.Clean(objdir+of+".d"))
            create_list = append(create_list,oname)

            carg := arg1
            for _,ca := range info.options {
                if ca == "$out" {
                    ca = oname
                } else if ca == "$dep" {
                    ca = dname
                } else if ca == "$in" {
                    ca = sname
                }
                carg = append(carg,ca)
            }

            cmd := BuildCommand{
                cmd : compiler,
                cmdtype : "compile",
                args : carg,
                infiles : []string{ sname },
                outfile : oname,
                depfile : dname,
                need_cmd_alias : true }
            my_list[i] = cmd
        }
        command_list = append(command_list,my_list...)
    }

    if NowTarget.Type == "library" {
        // archive
        if len(create_list) > 0 {
            arname := create_archive(info,odir,create_list,NowTarget.Name)
            result.create_list = append(info.create_list,arname)
            //fmt.Println(info.archive_options+arname+alist)
        } else {
            fmt.Println("There are no files to build.",loaddir)
        }
    } else if NowTarget.Type == "execute" {
        // link program
        if len(create_list) > 0 || len(info.create_list) > 0 {
            create_link(info,odir,append(create_list,info.create_list...),NowTarget.Name)
        } else {
            fmt.Println("There are no files to build.",loaddir)
        }
    } else if NowTarget.Type == "convert" {
        if len(cvfiles) > 0 {
            create_convert(info,loaddir,odir,cvfiles,NowTarget.Name)
        } else {
            fmt.Println("There are no files to convert.",loaddir)
        }
    } else if NowTarget.Type == "fallthrough" {
        result.create_list = append(info.create_list,create_list...)
    } else {
        //
        // other...
        //
    }
    result.success = true
    return result,nil
}

//
//
//
func output_rules(file *os.File) {
    file.WriteString("builddir = "+outputdir+"\n\n")
    file.WriteString("rule compile\n")
    file.WriteString("  command = $compile $options $in -o $out\n")
    file.WriteString("  description = Compile: $desc\n")
    file.WriteString("  depfile = $depf\n")
    file.WriteString("  deps = gcc\n\n")
    file.WriteString("rule ar\n")
    file.WriteString("  command = $ar $options $out $in\n")
    file.WriteString("  description = Archive: $desc\n\n")
    file.WriteString("rule link\n")
    file.WriteString("  command = $link $options -o $out $in\n")
    file.WriteString("  description = Link: $desc\n\n")
    file.WriteString("rule convert\n")
    file.WriteString("  command = $convert $options -o $out $in\n")
    file.WriteString("  description = Convert: $desc\n\n")

    for ar,arv := range append_rules {
        file.WriteString("rule "+ar+"\n")
        file.WriteString("  command = "+arv+"\n")
        file.WriteString("  description = "+ar+": $desc\n\n")
    }
}


//
// application interface
//
func main() {

    flag.BoolVar(&isRelease,"release",false,"release build")
    flag.BoolVar(&isDebug,"debug",true,"debug build")
    flag.StringVar(&target_type,"type","default","build target type")
    flag.StringVar(&target_name,"t","","build target name")
    flag.StringVar(&outputdir,"o","build","build directory")
    flag.Parse()

    if isRelease {
        isDebug = false
    }
    outputdir_set = false

    ra := flag.Args()
    if len(ra) > 0 && target_name == "" {
        target_name = ra[0]
    }

    append_rules = map[string] string{}
    command_list = []BuildCommand{}

    if target_name != "" {
        fmt.Println("gobuild: make target: "+target_name)
    }
    build_info := BuildInfo{
        variables : map[string] string{"option_prefix":"-"},
        includes : []string{},
        defines : []string{},
        options : []string{},
        archive_options : []string{},
        convert_options :[]string{},
        link_options :[]string{},
        create_list :[]string{},
        select_target : target_name,
        target: target_name }
    var r,err = build(build_info,"")
    if r.success == false {
        fmt.Println("gobuild: error:",err.Error())
        os.Exit(1)
    }

    nlen := len(command_list)
    if nlen > 0 {

        file,err := os.Create("build.ninja")
        if err != nil {
            fmt.Println("gobuild: error:",err.Error())
            os.Exit(1)
        }

        // execute build
        output_rules(file)

        for _,bs := range command_list {
            file.WriteString("build "+bs.outfile+": "+bs.cmdtype)
            for _,f := range bs.infiles {
                file.WriteString(" $\n  "+f)
            }
            for _,dep := range bs.depends {
                file.WriteString(" $\n  "+dep)
            }
            if bs.need_cmd_alias {
                file.WriteString("\n  "+bs.cmdtype+" = "+bs.cmd+"\n")
            } else {
                file.WriteString("\n")
            }
            if bs.depfile != "" {
                file.WriteString("  depf = "+bs.depfile+"\n")
            }
            if len(bs.args) > 0 {
                file.WriteString("  options =")
                for i,o := range bs.args {
                    if i & 3 == 3 {
                        file.WriteString(" $\n   ")
                    }
                    file.WriteString(" "+o)
                }
                file.WriteString("\n")
            }
            file.WriteString("  desc = "+bs.outfile+"\n\n")
        }

        fmt.Println("gobuild: done.")
    } else {
        fmt.Println("gobuild: empty")
    }
}
//
//
