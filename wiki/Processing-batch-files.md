# Processing batch files

This note does not document any new feature or API. It specifies a SOP by which batch files may be processed in any application which is built on Alya.

Almost all enterprise applications need to accept batch files, check them for syntax and validity, and process them. In Alya, it is recommended that the batch file be converted into an Alya batch, one line of the file becoming one record in the batch. This allows a partially processed batch to be resumed from the point where the last record was completed.

## Three incoming channels for files

Files may be uploaded into the system in three ways:
* by SCP
* by an incoming API call whose payload will include a string, which is the contents of the file
* from the UI, by a human user uploading a file using a browser

The three incoming channels will be processed in the following ways:
* **SCP**: the file will be stored in an `incoming` directory by the SSHd, and will be picked up by a CLI program. This program will then store the file in an object in the object store and call `bulkfilein_process()`. When calling this function, it will pass the object ID.
* **API**: the web service which will receive the file contents will call `bulkfilein_process()` and pass the file contents as a string
* **UI**: the web service which will receive the file contents will call `bulkfilein_process()` and pass the file contents as a string

The web service for the incoming API (for server-to-server calls from external entities) and for the application's UI (for access by front-end code in browsers and apps used by human users of the same organisation) may be the same or very similar. The decision to have a common WS or two separate WS will be taken case-by-case depending on differences in processing and authorization rules.

In our experience, all batch files which are received by any channel need to be archived for audit and forensic purposes. For all such file-storage purposes, Alya recommends the use of an object store instead of a file system. It is also necessary to maintain a batch-files table where the metadata of all such files are maintained, with a pointer to the object in the object store which has the actual contents.

## Handling files received over `scp`

The three modes by which files come in are `scp`, server-to-server API calls, and uploading from a UI screen (which internally becomes a web service call). In the second and third modes, each file and its type are clearly identified by the parameters which are passed with the web service. But in the first mode, files are transferred to a separate server and dumped there, to be picked up by a daemon.

To enable this infrastructure to work, the following setup is recommended:
* Each counterparty organisation which is expected to submit files by `scp` must be given a separate Unix account, SSH key, and home directory. Therefore, two organisations uploading files using `scp` will not be able to access or overwrite each other's files. Authentication by passwords **must never be used**, only key pairs must be used.
* Inside each SSH account's home directory, two subdirectories must be created, called `incoming` and `outgoing`, for incoming and outgoing files. All files uploaded to the application will appear in the `incoming` directory.
* File naming conventions must be followed by each counterparty organisation to indicate the file type and contents, in case one organisation submits many file types.

A daemon program, called `infiled`, will be developed. The source code of the generic portion of this program is part of Alya. The file-check functions and the main program will be written by the application developer.

This daemon program will read a JSON file to map files in the file system's `incoming` directories with file types. Once it finds a file, it will use this map to identify its file type. It will then write the file to the object store and call `BulkfileinProcess()` for the object.

The JSON map will be of the form
``` json
"filetypemap": [{
    "path": "/*/incoming/txnbatch/TXN*.xlsx",
    "type": "banktxnbatch"
}, {
    "path": "/*/incoming/paymtbatch*.csv",
    "type": "bankpaymtinfo"
}, {
    :
    :
}]
```
Each element in the array will be an object with two fields, `path` and `type`. The `path` will be the regex pattern of all files of a certain type in all directories, and the `type` field will specify the type of the files which match that pattern. 

The `infiled` program will run a Unix `find` command or something similar, once for each element in the `filetypemap` array, and pick up all files it finds more than `INBULKFILE_AGESECS` old which match the `path` pattern, and process them.
```
for each element in filetypemap do
    filelist = find all files which match the path pattern of the element
    for each file in file list do
        create an object in the "incoming" bucket in the object store
        call BulkfileinProcess() with the file and filetype
        if BulkfileinProcess() fails then
            move the object to the "failed" bucket in the object store
        endif
        delete the file
    endfor
endfor
```
The daemon will repeat this loop within an outer loop which will run infinitely, at intervals of `INBULKFILE_SLEEPSEC` seconds.

In this approach, whether a file is successfully queued for batch processing or fails to be processed, the file is deleted from the file system and is stored somewhere in the object store.

The source code for `infiled` will be in two parts:
* generic code which will be part of Alya, in the package `Alya.Batch`
* application-specific code which will be written by the application dev team.

The `main()` function will be written by the application dev team and will contain
* calls to `RegisterFileChk()`,
* calls to various functions to set various configuration parameters of `infiled`, and
* finally, a call to `Alya.Batch.InfiledLoop()`

The binary for `infiled` will be built by linking the `main()` function, all the source files of the various file-checking functions, and `Alya.Batch`. The function `Alya.Batch.InfiledLoop()` will never return.

## `BulkfileinProcess()`

This Go function will take
* `file`: a string with the file contents, or an object ID. If it's less than, say 40 chars long, then it's an object ID. Else it's a big string holding the file contents.
* `filename`: the (optional) file name, and
* `filetype`: an enum value for file type

and return `error`.

Internally, it'll be a big case statement keyed by file type, where it will call the `filechkXXX()` function for that type.

```
if an object ID was given in the request, not the file contents, then
    read the object contents into an in-memory string
endif
switch filetype do
case filetype 1: call the filechk_type1() function
case filetype 2: call the filechk_type2() function
case filetype 3: call the filechk_type3() function
    :
    :
endcase
if the filechk() function reported "ok" then
    submit the array of records to a batch using Alya.Batch.Submit() calls
endif
if the file contents were given in the request, not an object ID, then
    write out the file contents into an object in the object store
endif
write a record in the batch-files table with
        the object ID, object size, checksum, the received-at timestamp, and
        status indicating whether the filechk() failed or succeeded
return

```
The `Alya.Batch.Submit()` function is documented [here](https://github.com/remiges-tech/alya/wiki/Slow-queries-and-batches#alyabatchsubmit) for reference

## The `filechk()` functions

There will be one `filechk()` function for each file type.

The `filechk()` functions will take as input
* a string with the file contents
* a string with the name of the submitted file

and will return
* a boolean, `isgood`, to indicate whether the file may be submitted for batch processing or is entirely garbage.
* the `context` JSON string to pass to `Batch.Submit()`.
* the `batchinput` array of objects to pass to `Batch.Submit()`.
* other parameters which are needed to make the calls to `Batch.Submit()`.
* an object ID of an object containing an error file, in case the error-check failed for any of the lines.

The output parameters `context`, `batchinput`, and other parameters needed for `Batch.Submit()` will be `nil` if `isgood` is `false`. But the output object ID of the error file may be `nil` or may contain a valid object ID with the error file.

If some of the lines are invalid, `isgood` will be `true`, and the `input` array will exclude the faulty rows. In that case, it must be noted that the line numbers specified in the objects of the `input` array **must correspond to the line numbers in the source file** and must not be blindly given as sequential integers.

The value of `isgood` will be `false` only if the file was totally garbage, *e.g.* the number of columns expected per line were not present, *etc*

This function will access Redis to check that there are no duplicates of either the file or any of the rows. It will access Rigel to find out how to connect to Redis. A duplicate row will be treated as a faulty row. A duplicate file will result in `isgood` being set to `false`.

## The `RegisterFilechk()` function

This function will register a file-checking function against each file type. If the application needs to process batch files of 55 types, then 55 file-checking functions will be registered in the system by calling `RegisterFilechk()` 55 times. Later, `BulkfileinProcess()` will look up the correct file-checking function for each file type and call it.

The parameters for the file-checking functions have been discussed earlier. These functions just need to take the file contents and do their checking. They have many return parameters.

``` go
// the FileChk function:
// INPUT param:
//     string: file contents
//     string: file name
// RETURN params:
//     bool: isgood, 
//     JSONstr: context, 
//     []BatchInput_t: batch_input
//     string: app
//     string: op 
//     string: error_file's object ID

type FileChk func(string, string) (bool, JSONstr, []BatchInput_t, string, string, string)

//create map to register
var fileChkMap map[string]FileChk

func RegisterFileChk(filetype string, fileChkFn FileChk) {
      fileChkMap[filetype] = fileChkFn 
}
```

This `RegisterFileChk()` function maintains the file-type-to-function map in a private global map structure, which is accessed only by `BulkfileinProcess()`.