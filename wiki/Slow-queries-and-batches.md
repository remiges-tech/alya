Slow queries must be processed asynchronously.

These things may not even be queries -- they may be updates, but may take a lot of time (in excess of 10 seconds). Such operations too will be processed through a slow query framework, which is a part of Alya.

# Architecture for slow queries

The basic apparatus in Alya for supporting slow queries will be a set of asynchronous threads in each Alya instance, which will poll a work queue, receive jobs, and execute them. Therefore, a cluster of containers running Alya will have a shared database with two tables.

The steps will be as follows:
1.  When a client application invokes a WSC which is implemented as a slow query, this WSC will do the following:
    1.  Insert a new record in the `batches` table with a unique ID called a "request ID", and insert a new record in the `batchrows` table.
    1.  Respond to the caller with a response which includes the request ID, and asks the caller to "try later".
1.  The client application then keeps polling the WSC every several seconds, supplying the request ID instead of the original parameters
1.  In the meanwhile the record in `batchrows` is picked up by one of the asynchronous processing threads in one of the Alya instances in the cluster, and this thread then looks at the "op" field of the record and calls the handler function for that type of "op" (for operation type). The asynchronous threads run a top-level function referred to below as a job manager, and this then calls "op"-specific handler functions. These handler functions are registered with Alya using `RegisterProcessor()` calls.
1.  The asynchronous thread at some point finishes processing the request, and the handler function returns to the job manager, which updates the record in the `batches` and `batchrows` tables with the appropriate results. Optionally, it also creates blobs, stores them in the object store, and notes their IDs in the `batches` record.
1.  The next time the client calls the WSC with this request ID, the WSC looks up the record in `batches`, finds that the task is complete, pulls out the results and sends them to the client.

# Architecture for batch processing

Batches are a set of processing operations where one algorithm is applied to process each instance of a large set of input data blocks. This is conceptually identical to calling one function over and over with various sets of input parameters. Batch processing is a requirement of most large business applications, and it is important to
* distribute the processing operations across many servers if a cluster of servers is available, to complete a large batch in less time
* track the progress of each unit such that if some of the unit operations are interrupted due to a partial system failure, the batch may be resumed or its unit operations re-distributed to healthy nodes

A slow query, as defined by Alya, is a single operation which may take too long to be executed synchronously in one WSC. (Typical WSC response time expectations are at sub-second levels in most systems in the 21st century.) A batch is a (large) collection of operations, each of which may or may not be fast enough to be executed synchronously in a WSC, but as a whole is too time-taking to be done synchronously and in terms of latency expectations, may be queued and executed to distribute overall system load across servers. Therefore, Alya provides a batch processing framework which incidentally is also used for slow queries.

In the Alya batch processing framework, a WSC (or other component of the application, *e.g.* a CLI program) submits an entire batch to Alya. After this, it polls Alya every several seconds to check if the batch is complete.

For batch processing, the business application makes one web service call and submits an entire batch of jobs to Alya. Alya then uploads the array of jobs into its `batchrows` table. All the asynchronous processing threads in the cluster pick up these rows in random order, in blocks or chunks, as soon as they have some free resources. Each chunk typically comprises 10-100 jobs. Each processing thread completes its jobs and updates the results in the `batchrows` and then `batches` tables maintained by Alya. Finally, when the entire batch is complete, Alya consolidates the results and gets ready to hand the full batch results back to the application code whenever it is asked.

A slow query fits into this batch processing framework easily. It becomes a batch with one job. Everything else remains exactly the same as for a large batch. A batch may be viewed as an array of slow queries with a common tag called a batch ID, and a common processing operation.

# Database tables

In case it was not clear till now, note that these database tables will be in a database which will be for the private use of Alya. They will not necessarily be part of the business application databases.

## The `batches` table

This table will contain
* `id`: mandatory, a unique ID of the request or batch. Typically, this will be a UUID which the WSC will generate and insert
* `app`: mandatory, one-word string following the syntax rules of PL identifiers, all lower-case, identifying the application which this request belongs to. This allows the batch processing framework to be a shared service serving multiple applications.
* `op`: mandatory, string, which names one of the different processing functions which are registered with this framework. (A processing function may be registered with the framework by a call to `Alya.SlowQuery.RegisterProcessor()` or `Alya.Batch.RegisterProcessor()` by the application code.) If the application has three types of slow queries, then this field will have one of their three names. `op` means `operation`, and identifies which operation this request is asking to be performed. These operation names will be single-word, all-lower-case, corresponding to the syntax rules of identifiers in modern PL.
* `type`: mandatory, single character, `Q` for slow query, `B` for batch.
* `context`: mandatory, a JSON block containing the user identity information and other context information if any. May be empty block.
* `inputfile`: optional, string, filename. May be any string, Alya does not process this data. It is useful to store the name of a physical file or any other string to identify a physical file, email, or data block submitted to the application from some external source to trigger this batch. Note that this is expected to hold something of a few dozen or a few hundred bytes, like a file *name*, not a few megabytes like the contents of a datafile. Example: `"SIP-EOD-2024nov23_2330"`
* `status`: mandatory, enum, may have one of the six values `wait`, `queued`, `inprog`, `success`, `failed` or `aborted`. Initially, a new record will have the value `wait` or `queued`. When a processing function picks up one of the rows in `batchrows` corresponding to this batch, the status will change to `inprog`. The value will be changed to `success` after all the rows have been completed and there are zero errors, and will be set to `failed` after all the rows have been completed and one or more rows have `status == failed`. It will have the status `aborted` if the batch was aborted.
* `reqat`: mandatory, timestamp specifying the time at which the WSC enqueues this request
* `doneat`: initially `NULL`, contains the timestamp at which all rows of this operation are completed by the asynchronous processing function
* `outputfiles`: initially `NULL`, contains a JSON object giving the logical names and object IDs of the output blobs or files generated by the batch run. Will be `NULL` even after the batch completes if no output files are generated. A slow query does not need to use this feature mandatorily; it can return a reference to output files/blobs in one of the fields of the `result` block of the `batchrows` record. The value returned in the `outputfiles` parameter of `SlowQuery.Done()` or `Batch.Done()` will be picked up from this field of the record, unmarshalled and returned.
* `nsuccess`: integer, initially `NULL`, indicating how many rows of this batch have status `success` after the batch is complete.
* `nfailed`: integer, initially `NULL`, indicating how many rows of this batch have status `failed` after the batch is complete.
* `naborted`: integer, initially `NULL`, indicating how many rows of this batch have status `aborted` after the batch is complete. This value will be non-zero only if the batch as a whole has status `aborted`.

The JSON object in `outputfiles` will have a set of one or more members of the form
```
{
    "tradesdone": "42253a86-ba96-11ee-a6b5-d316d473fc73"
    "errlist": "5060de66-ba96-11ee-af79-0ffa0d2bb863"
}
```
where, in the above example, two output files are generated at the end of the batch run. One has a logical name `tradesdone` and the value is an object ID for the file in the system's object store or a path to a file in a shared file system. The second has a logical name `errlist` with another ID or file path of another object. Each member of this JSON object will have a logical name indicating the purpose of that file (a purpose known only to the application code, not Alya) and a value which will be an object ID or file path. The application code may do what it needs to with this information.

## The `batchrows` table

This table will contain
* `rowid`: mandatory, integer, internally generated unique auto-incrementing
* `batch`: mandatory, string, the ID from the `batches` table
* `line`: mandatory, integer, giving the line number of this row from an input batchfile, in case a batchfile was used to fire the batch. In the case of slow queries, this value will be `0`.
* `input`: mandatory, a JSON block containing the input parameters passed by the client to the WSC. May be an empty JSON block.
* `status`: mandatory, enum, may have one of the five values `queued`, `inprog`, `success`, `failed` or `aborted`. Initially, a new record will have the value `queued`. When a processing function picks up the request, the status will change to `inprog`. And so on.
* `reqat`: mandatory, timestamp specifying the time at which the WSC enqueues this request and inserts this record in this table
* `doneat`: initially `NULL`, later contains the timestamp at which this operation is completed by the asynchronous processing function
* `res`: initially `NULL`, the JSON block holding the results of the operation. Any result too big to fit into a "reasonable sized" WSC response will go into objects in an object store. If `messages` is not `NULL`, then this field will be `NULL`.
* `blobrows`: initially `NULL`, this will be a JSON array with logical blobname (or filename, if we want to use older-generation terms) and row to be appended to that blob from this record. There may be no `blobrows` associated with this operation, so this field may remain `NULL`.
* `messages`: initially `NULL`. At the end of the operation, this may be a JSON block with an array of errors generated by the request due to which the operation failed. If `res` is not `NULL`, then this field will be `NULL`.
* `doneby`: initially `NULL`, a string in some format which indicates the server or container or Alya instance which processed this row. This value is set when a record is picked up for processing and its status is changed from `queued` to `inprog`.

The idea of `blobrows` is to allow each row's processing to specify some ASCII text which must be appended to a text file. (Text files are stored as blobs in the system's object store.) The processing function can thus contribute a line or two to an aggregate batch output file which will be constructed from all the `blobrow` entries once the entire batch is processed. The `blobrows` JSON block will have a structure of this format:
``` json
{
    "outputfile": "line 3269: 25000,8953,\"Axis Mutual Fund\",\"2024-02-03T10:53:05Z\""
    "errfile": ""
}
```
Basically, it will an object with a collection of field names and values. All values will be text strings. The example above indicates that this row from the batch wishes to append the string
```
line 3269: 25000,8953,"Axis Mutual Fund","2024-02-03T10:53:05Z"
```
to the blob or file referred to by the logical name `"outputfile"`, and an empty line to `"errfile"`. There is no specific limit to the number of logical blobs to which lines may thus be appended. Each blob named in `blobrows`, *e.g.* `outputfile` and `errfile` here, will finally become a text file and will be stored in the object store as a blob. There is no restriction on the text which may be included as the value of a member in `blobrows`, UNICODE runes are accepted too, and therefore if a linefeed character is embedded in a string, then it will result in two lines being added to the blob finally. If the value is a zero-length string, then a blank line will be added for this row to the output blob. If a record intends to not add any line to a specific blob, the `blobrows` for that record must not contain any field for that blob. Therefore, it implies that not all `blobrows` for a given batch will contain the same set of members.

The members of the `blobrows` object will finally match the members of the `outputfiles` JSON object in the `batches` table.

The `line` column will have the value `0` and the `blobrows` column will be `NULL` for a slow query record. These columns only become useful for batches.

# Client API

This API defines the functions exported by the Go library which will be used by the WSC to
* submit a slow query to Alya,
* query Alya about the status of a request,
* submit a batch (lots of rows),
* query Alya about the status of the batch, and
* submit a processing function which Alya's asynchronous threads will call if a job of that matching type is submitted

``` golang
type JSONstr string
```

## `Alya.SlowQuery.Submit()`

This function used by the application code to submit a slow query to the batch processing framework.

``` golang
func (S SlowQuery) Submit(app, op string, context, input JSONstr)
        (reqID string, err error)
```

**Request**

All the parameters are mandatory.
* `app`: gives the one-word name of the application whose processing function is to be invoked
* `op`: gives the name or label of the operation. Alya will invoke the processing function associated with this name or label.
* `context`: gives the JSON version of the context data to be used for processing the query. This JSON block will be stored in the `batches` table and will be picked up by the processing function. It may include the user's authorisation credentials, the IP address she's invoking this WSC from, and any other information the processing function thinks is useful.
* `input`: used in ways analogous to `context`. This is the set of fields received by the WSC in its request, which will directly be useful for processing the request.

Note that the caller can submit a big input file for processing to the slow query. The caller can load the file into the object store (which we always presume is present) and pass the object ID as one of the fields in `input`, or, if there is a shared file system between the caller and the Alya processes, then it can store a file in this file system and pass the file path in `input`. Alya will not know or care what happens to the blob or file -- the processing function can delete the object, archive the item in AWS S3 Glacier, delete it, or anything else.

**Processing**

This function will generate a unique request ID, write a record in `batches` with `type` set to `Q`, and a record in `batchrows`.

**Response**

* `reqID`: if all goes well, the function will return a unique string ID of the queued request
* `err`: error if any, as per the Go convention

The `err` object will carry various errors including errors in the input parameters, errors due to component failure (*e.g.* database access failure, *etc*) and so on.

## `Alya.SlowQuery.Done()`

This function is used by the application code to check if a slow query is complete or not. This is a polling request, and ideally must be called only at intervals of several seconds.

``` golang
type BatchStatus_t int
const(
    BatchTryLater BatchStatus_t = iota
    BatchWait
    BatchQueued
    BatchInProgress
    BatchSuccess
    BatchFailed
    BatchAborted
)
func (S SlowQuery) Done(reqID string)
            (status BatchStatus_t, result JSONstr, messages []Alya.ErrorMessage, outputfiles map[string]string, err error)
```

**Request**

The input parameter is self-evident

**Processing**

The function will check REDIS to see if there is any entry with a key which looks like this:
```
ALYA_BATCHSTATUS_e7a6f2b0-ba91-11ee-a1e9-1ff5fe17fe87
```
where the substring after `BATCHSTATUS` is the request ID of the slow query. If such a key exists, then the value will be any of the five values used in the `status` column of `batches`. If such a key exists, the code will decide its action based on this value. Else it will look up the record in `batches` to see if `type` is `Q` and then see if `status` is `success`, `failed` or `aborted`. If it is `success` or `failed`, it will return the data from the corresponding `batchrows` table in the format specified and set `status` to `BatchSuccess` or `BatchFailed` as the case may be. In the case of an aborted batch, there will be no useful data in `batchrows`, therefore no data will be retrieved from that table and returned; `status` will be set to `BatchAborted`. Else if the value is not any of these three, the function will return `status` as `BatchTryLater`, and will insert a record into REDIS with the key in the format given, the value as the status as extracted from the database, and an expiry time of duration configurable *via* the global config parameter `ALYA_BATCHSTATUS_CACHEDUR_SEC`. This figure will be an integer number of seconds, typically of the order of 15-60.

NOTE: if `batches.status` is `success`, `failed` or `aborted` and no REDIS record was found for this batch, then the new REDIS record inserted will have an expiry time 100x of the number of seconds specified in the global config parameter `ALYA_BATCHSTATUS_CACHEDUR_SEC`.

**Response**

The output parameters will be defined depending on whether `status` says `BatchTryLater` or not. If it says `BatchTryLater`, then none of the other parameters will carry useful data. If it carries some other value, then
* `result` will carry a JSON data structure which will be unmarshalled by the caller to extract the result of the processing. It's possible that some of the fields in `result` may contain object IDs in the object store, in case the processing function generated large data objects like reports as output. `result` will be `nil` if `messages` carries processing errors.
* `messages` will be an array of error messages in the `ErrorMessage` format supported by Alya (see [this file in the source](https://github.com/remiges-tech/alya/blob/a27d7e210496da22ef824dd9062b0e4a664d05ac/wscutils/wscutils.go#L43) for full details). This array will be empty if there is no error.
* `err` will carry an error status as per Go convention. There is no correlation between the `status` field of the `batches` record having the value `success` and the value of this parameter. This parameter will indicate an error only in case of some sort of critical system error (*e.g.* database access error for Alya's private database). If the error is at the level of business logic, then those errors will be reported in `messages`, not in `err`.

The calling function must process the response in the following sequence:
```
if err != nil then
    log CRITICAL error
    return
endif
if status == BatchTryLater or BatchAborted then
    return status
endif
if status == BatchSuccess then
    unmarshall the "result" object and do your work
else
    unmarshall the "messages" array and process the errors
endif
return
```
The slow query may return files, either *via* a shared object store or *via* a shared file system, and just include the file path or object ID in the `result`.

## `Alya.SlowQuery.Abort()`

This function is useful when an application has fired a slow query and realises that there is no need for the query to be processed.

``` golang
func (S SlowQuery) Abort(reqID string) (err error)
```
**Request**

The input parameter is self evident.

**Processing**

The function will check that the `batches` record for this ID has `type` set to `Q`, then will update the record for this slow query in `batches` and `batchrows` and set the `status` to `aborted`, provided that the status of the job in `batches` is not `aborted`, `success` or `failed`. The function will start by doing a BEGIN TRANSACTION, then SELECT FOR UPDATE across both tables for all records which have the request ID. Then it will inspect the `status` and `doneat` fields of the records and update them if needed; the `status` will be set to `aborted` and `doneat` to current timestamp.

Set the REDIS batch status record for this batch to `aborted` if the records in the database have been updated now. The REDIS record expiry time should be 100x of the number of seconds specified in the global config parameter `ALYA_BATCHSTATUS_CACHEDUR_SEC`.

**Response**

If there was any systems failure or if the request ID was invalid, then `err` will have a non-`nil` value. Else all will be well.

## `Alya.SlowQuery.List()`

List all the slow queries currently being processed or processed in the past against a specific `app`.

``` golang
type SlowQueryDetails_t struct {
    id, app, op, inputfile    string
    status                    BatchStatus_t
    reqat, doneat             time.Time
    outputfiles               map[string]string
}
func (S SlowQuery) List(app, op string, age int)
        (sqlist []SlowQueryDetails_t, err error)
```

**Request**

The function will take the following input parameters:
* `app`: mandatory, string, specifying the app for which the slow queries will be returned
* `op`: optional, string, will return only those slow queries whose `op` matches the value
* `age`: mandatory, integer, must be greater than 0. This gives the age of the records to return, checking backward in time from the current time. A value of `7` here will return the records whose `reqat` timestamp is less than seven days old.

**Processing**

The call will pull out details from the `batches` table for all matching records whose `type` is `Q`, and return in an array. Note that the response may be quite a large array if the tables have not been purged of old records.

**Response**

The response will have a non-`nil` value for `err` if there was some system error like a database access error. Else, the array `sqlist` will have zero or more entries.

## `Alya.Batch.Submit()`

This function is used by the application code to submit a batch to the batch processing framework.

``` golang
type BatchInput_t struct {
    lineno int
    input JSONstr
}
func (B Batch) Submit(app, op string, context JSONstr, batchinput []BatchInput_t, waitabit bool)
        (batchID string, err error)
```
where
* `app`, `op`, and `context` are the same as for `SlowQuery.Submit()`
* `batchinput` contains all the data for all the rows of the batch. This array must have at least one entry, and at most the number specified in the global config `ALYA_BATCHROWMAX`. (Typically, this value will exceed 100,000.) All the `lineno` values must be greater than `0`. It is acceptable if `batchinput` has only one element.
* `waitabit` will, if `true`, prevent the batch from being picked up immediately and processed once this call returns. This flag can be switched off later by a `WaitOff()` call or by an `Append()` call with `waitabit` set to `false`. Or else, it may be `false` in this call itself, if the caller knows that this is not the first round of a multi-round batch submission. If the caller knows that more rounds of batch records are to be submitted, then this parameter **must** be set to `true`, else the subsequent calls to `Batch.Append()` will fail.

**Processing**

This function will generate a unique batch ID, write a record in `batches` and many records in `batchrows`. The rows in `batchrows` will carry `line` and `input` from the corresponding elements of `batchinput`.

The status of the `batches` record will be set to `wait` if `waitabit` is `true`, and to `queued` if it was `false`.

**Response**

The response will include the batch ID if `err` is `nil`.

## `Alya.Batch.Append()`

This function is used by the application code as a follow-up to `Alya.Batch.Submit()` for long batches where the setting up the batch in multiple rounds is easier.

``` golang
type BatchInput_t struct {
    lineno int
    input JSONstr
}
func (B Batch) Submit(batchID string, batchinput []BatchInput_t, waitabit bool)
        (batchID string, nrows int, err error)
```
where
* `batchID` is a string specifying the batch ID to append these rows to
* `batchinput` contains all the data for all the rows of the batch. This array must have at least one entry, and at most the number specified in the global config `ALYA_BATCHROWMAX`. (Typically, this value will exceed 100,000.) All the `lineno` values must be greater than `0`. It is acceptable if `batchinput` has only one element.
* `waitabit`, if `true` tells the service that the batch is not yet fully submitted, or for some reason is to be held back till further notice. If it is `false`, it indicates that this `Append()` is the last round and the batch has now been completely uploaded and may be processed immediately.

**Processing**

This function will check whether a batch record exists in the `batches` table with the ID given. If yes, the status of the batch must be `wait`; all other status will generate an error. The function will then write many records in `batchrows`. The rows in `batchrows` will carry `line` and `input` from the corresponding elements of the `batchinput` array.

Once this is done, the status of the `batches` record will be changed from `wait` to `queued` if `waitabit` is `false`. If not, the status will be left unchanged.

**Response**

The response will include the batch ID and total count of rows in `batchrows`, if `err` is `nil`.

## `Alya.Batch.WaitOff()`

This call sets the status of a batch from `wait` to `queued`.

``` golang
func (B Batch) WaitOff(batchID string) (batchID string, nrows int, err error)
```

**Request**

The input is self-evident.

**Processing**

The function will do a SELECT FOR UPDATE to lock the row in `batches`, then check if the status of that row is `wait`, and if it is, will change the status to `queued` and then commit the transaction. It will trigger an error if the batch record does not exist or if it has any status other than `wait` or `queued`.

If the status of the row is already `queued`, the call will just return success.

**Response**

The batch ID and the total number of rows in `batchrows` against this batch will be returned, if `err` is `nil`.

## `Alya.Batch.Done()`

This function will be used by the application code to poll the batch processing framework and check if a specific batch is complete or not. Calling this function does not change any state of any processing or data, other than the occasional insertion of a cache entry.

``` golang
type BatchOutput_t struct {
    line     int
    status   BatchStatus_t
    res, messages JSONstr
}
func (B Batch) Done(batchID string)
            (status BatchStatus_t, batchOutput []BatchOutput_t, outputFiles map[string]string, 
             nsuccess, nfailed, naborted int,
             err error)
```

**Request**

The input parameter is self evident.

**Processing**

The function will first check REDIS for an entry with a key which looks like this:
```
ALYA_BATCHSTATUS_e7a6f2b0-ba91-11ee-a1e9-1ff5fe17fe87
```
where the substring after `BATCHSTATUS` is the batch ID. If the key is present, its value will be used for decision making, else the `batches` table will be queried for the given batch ID, `batches.status` will be extracted, and updated in REDIS with an expiry duration governed by the global config parameter `ALYA_BATCHSTATUS_CACHEDUR_SEC`.

* If `batches.status` is `aborted`, `success` or `failed`, then the `batchrows.line`, `batchrows.status`, `batchrows.res` and `batchrows.messages` will be extracted and constructed into a single array called `batchOutput` where each element is of type `BatchOutput_t`. Each element in this array will contain the values of `status`, `line`, `res`, and `messages` from one record of `batchrows`. The `res` and `messages` members will be JSON blocks. The structure of each `res` block is application specific, but the structure of each `messages` object is as per the Alya standard for returning messages in WSC responses.

    Therefore if the batch had 100,000 rows, then the `batchOutput` array will now have 100,000 elements. Some elements will have only `res` strings because their processing succeeded, and others will have only `messages` because their processing failed; none will have both. The overall batch status of `failed` means that one or more rows failed; an overall status of `aborted` means that some of the rows were aborted. If the overall status is `aborted`, then some rows may have `status` of `BatchAborted`, and in those cases, both `res` and `messages` will be `nil`, but `line` will have a meaningful value.

    The `outputFiles` parameter returned will be a straight lift from `batches.outputfiles`. It will be assumed that the application code will have access to the system object store and can extract the blobs from there using the object IDs.

* If `batches.status` is `wait`, `queued` or `inprog` then `status` will return `BatchTryLater`.

NOTE: if `batches.status` is `success`, `failed` or `aborted` and no REDIS record was found for this batch, then the new REDIS record inserted will have an expiry time 100x of the number of seconds specified in the global config parameter `ALYA_BATCHSTATUS_CACHEDUR_SEC`.

**Response**

The return parameters are
* `status` which will carry one of the values of `BatchStatus_t`.
* If `status` is `BatchAborted`, `BatchFailed` or `BatchSuccess`, then
  * `batchOutput` will carry useful data, one entry per record of the batch
  * `outputFiles` will specify a set of zero or more blobs
  * `nsuccess`, `nfailed` and `naborted` will carry counts of how many records of each status are there in the result set. `naborted` will have value `0` unless the overall `status` is `BatchAborted`.
* `err` will indicate the presence of any critical system error which prevented the function call from completing cleanly, or basic input error like invalid batch ID.

## `Alya.Batch.List()`

List all the batches currently being processed or processed in the past against a specific `app`.

``` golang
type BatchDetails_t struct {
    id, app, op, inputfile    string
    status                    BatchStatus_t
    reqat, doneat             time.Time
    outputfiles               map[string]string
    nrows, nsuccess, nfailed, naborted   int
}
func (B Batch) List(app, op string, age int)
        (batchlist []BatchDetails_t, err error)
```

**Request**

The function will take the following input parameters:
* `app`: mandatory, string, specifying the app for which the batches will be returned
* `op`: optional, string, will return only those batches whose `op` matches the value
* `age`: mandatory, integer, must be greater than 0. This gives the age of the records to return, checking backward in time from the current time. A value of `7` here will return the records whose `reqat` timestamp is less than seven days old.

**Processing**

The call will pull out details from the `batches` table for all matching records whose `type` is `B`, and values for `nrows` from the `batchrows` table, and return in an array. Note that the response may be quite a large array if the tables have not been purged of old records.

**Response**

The response will have a non-`nil` value for `err` if there was some system error like a database access error. Else, the array `batchlist` will have zero or more entries.

## `Alya.Batch.Abort()`

This function is useful if a batch needs to be aborted after it has been submitted.

``` golang
func (B Batch) Abort(batchID string) (err error)
```
**Request**

The input parameter is self-evident.

**Processing**

```
look for REDIS record for batch status. If the status is success/failed/aborted then
    log with INFO priority that the abort failed
    return with "err" set to some suitable error
endif
begin transaction
SELECT FOR UPDATE the batches record for this batch, fetch the status field
SELECT FOR UPDATE all records in batchrows for this batch
if batches.status is either "success" or "failed", then
    log with INFO priority that the abort failed
    rollback transaction
    return with "err" set to some suitable error to indicate this
endif
UPDATE the batches record, set status = "aborted" set doneat = current time
UPDATE the batchrows records for those where status == "queued" or "inprog",
         set status = "aborted", doneat = current timestamp
commit
set the REDIS batch status record for this batch to "aborted" with an expiry time
        100x of the number of seconds specified in ALYA_BATCHSTATUS_CACHEDUR_SEC
```

**Response**

The response will have `err` set to `nil` if all goes well, else will have some non-`nil` value otherwise. One cannot abort a completed batch, *i.e.* a batch where `doneat` has been set.

# Callback functions

## The `RegisterProcessor()` functions

``` golang
type SlowQueryProcessor interface {
    func DoSlowQuery(InitBlock any, context JSONstr, input JSONstr)
            (status BatchStatus_t, result JSONstr, messages []ErrorMessage,
             outputFiles map[string]string, err error)
    func MarkDone(InitBlock any, context JSONstr, sq SlowQueryDetails_t) (err error)
}
type BatchProcessor interface {
    func DoBatchJob(InitBlock any, context JSONstr, line int, input JSONstr)
            (status BatchStatus_t, result JSONstr, messages []ErrorMessage,
             blobRows map[string]string, err error)
    func MarkDone(InitBlock any, context JSONstr, batch BatchDetails_t) (err error)
}
func (s SlowQuery) RegisterProcessor(app string, op string, p SlowQueryProcessor)
            (err error)
func (b Batch)     RegisterProcessor(app string, op string, p BatchProcessor)
            (err error)
```
The application code will need to include an implementation of the `SlowQueryProcessor` or `BatchProcessor` interface for every processing function it requires.

This processing function will get
* `initblock` as a Go data structure returned by the `ProcessInit()` function for this application. See `RegisterProcessInit()` and `RegisterProcessInit()` functions below.
* `context` as a JSON block whose structure only the application code knows.
* `input` as another JSON block.
* *(for batches)* `line` which specifies the (notional) line number in a (notional) input batchfile from where this job came

This information will pertain to what we call "one row" or "one record of a batch". The processing function must open database connections to the application databases as needed and process this data, update business data records, *etc* as if it is a standalone CLI program. It will not get access to any Alya databases, therefore will not access `batches` or `batchrows`.

The function will return
* `status`: which will have the value either `BatchStatusSuccess` or `BatchStatusFailed`, nothing else
* `result`: a string holding a JSON block in a structure which makes sense to the business application code. Alya will store this block in `batchrows` but will not inspect or process it. May be `nil` if the operation failed and no results are to be returned.
* `messages`: an array of [`ErrorMessage` objects ](https://github.com/remiges-tech/alya/blob/a27d7e210496da22ef824dd9062b0e4a664d05ac/wscutils/wscutils.go#L43). May be `nil` if there are no errors and everything was a grand success.
* (for slow queries) `outputfiles`: a hashmap of logical file names and their object IDs in the object store. May be `nil` if no output files are created
* (for batches) `blobrows`: a hashmap of logical file names and the strings to be added to those files at the end of the full batch, to create output files. May be `nil` if this row of the batch does not contribute any lines to any output files.
* `err`: will be `nil` in typical cases when there were no system errors, and will carry an error if the call could not complete correctly. NOTE that the errors reported by this parameter are not at all related to the errors reported by `messages`. If `err` is not `nil`, it must be assumed that `resultJSON` and `messages` both may be half-complete, or undefined, or useless. They must not be processed at all.

The `SlowQuery.RegisterProcessor()` and `Batch.RegisterProcessor()` functions allow the business application to register their processor functions at startup time. If there are five kinds of batches and three kinds of slow queries in a business application, then five calls will be made to `Batch.RegisterProcessor()` and three to `SlowQuery.RegisterProcessor()` at startup, in each program running these asynchronous threads. This means that there must be eight different `types` defined in the code which implement the `SlowQueryProcessor` or `BatchProcessor` interface.

## The MarkDone functions

These callback functions will be called by the job manager whenever a slow query or a batch completes. There will be one callback function registered for each `(app, op)` tuple.

Any type which implements the `SlowQueryProcessor` or `BatchProcessor` interface must provide a method called `MarkDone()`. For a slow query, its `MarkDone()` callback will be called when the slow query completes.

What this function will do is decided by the business application developer. The function may make entries in databases, send out alerts, push out output files to remote locations, *etc*. It is possible for a business application to design its batch processing such that it does not call `SlowQuery.Done()` or `Batch.Done()` at all, and instead, just depends on the `Markdone()` callback functions to handle the post-processing steps. It may also choose to use both the `Done()` polling and the `MarkDone()` callbacks, in any combination necessary.

## The initializer functions

The need for initialization becomes clear once the processor functions are understood. A processor function is called by Alya once per row in a batch. Therefore, if a batch has 100,000 rows, the processor function is called 100,000 times. Typically, each invocation will require opening database connections, *etc*, which are magic incantations known to only the application designer, not to Alya. If these connection-open and connection-close operations are all done inside the processing function, it will slow down the processing. And since Alya's batch processing layer is completely agnostic to the business application, Alya cannot open database connections and hand over the open handles to the processing function. Hence, the developer who writes the processing function will need to write initialization functions too, and register them with Alya. Alya will call these functions once per `app`.

If there are 100,000 rows in a batch, and chunks are being carved out of the batch of 100 rows each, then Alya will do the following:
```
create the chunk of 100 rows
extract the app from these rows
call the init function for the app, create an InitBlock for the app
for each row in the chunk do
    call the processing function with the row and the initblock as parameters
endfor
call the initblock-close function for the app
```
This approach allows the init function to open database connections, initialize logging handles, load caches, *etc*, once, at the start of the chunk, and then pass those handles and resources to every row's processing function. The resources are released by calling the initblock-close function.

For each `app` which has batch processing or slow queries, the application developer will need to write:
* one data type which implements the `Initializer` interface, which has one function which creates and returns an `InitBlock`. (In OO parlance, `Initializer` is a static class which just has one method, `Init()`, which is a constructor for an `InitBlock` instance.)
* one data type which implements the `InitBlock` interface, which has two methods, `HealthCheck()` and `Close()`. (In OO parlance, `InitBlock` is a singleton class.)

Then, when each Alya instance starts up, the application developer will need to call `RegisterInitializer()` and pass it one instance of a variable which is of a type which implements the `Initializer` interface. Alya will call the `Init()` function of the `Initializer` object at the time, instantiate an instance of type `InitBlock`, and pass that `InitBlock` instance to every invocation of `DoSlowQuery()` and `DoBatchJob()`.

``` golang
type BatchInitializer interface {
    func Init(app string) (BatchInitBlock, error)
}
type BatchInitBlock interface {
    func IsAlive() (bool, error)
    func Close() (error)
}
func RegisterInitializer(app string, initializer BatchInitializer) (err error)

```

**Note**:

* there is only one `BatchInitBlock` per app, and is used by both slow queries and batches of that app
* the `BatchInitBlock` data structure will potentially be accessed by concurrent threads, if an Alya instance implements multiple threads to process rows of batches. If this happens, any updates to the shared data structure must manage concurrency, and this concurrency management is the responsibility of the application code (the processor functions). The `IsAlive()` and `Close()` methods of a specific object instance may be called by multiple concurrent threads. They must therefore be thread-safe.
* the `BatchInitBlock` object will have an `IsAlive()` method, which will inspect if all the handles and other references in that `BatchInitBlock` object are still valid. For instance, if database handles were stored there, are the DB handles still valid or have they timed out? This function will be called by the job manager when an old `BatchInitBlock` object is to be used again after a possibly long gap. The method must return just a `true` indicating it is alive and healthy, or `false` indicating that this object needs to be a closed and a new one allocated and initialised. If the application developer is in doubt about what to write in this `IsAlive()` method, she must just write a function to return `(false, nil)`.
* the `Close()` function will close an open `BatchInitBlock()`. Typically, the "close" operation means that database handles will be closed, other resources will be released, *etc*. If absolutely nothing needs to be done to "close" an init-block, this method could be an empty function just returning `nil` (for the `error`).

# The job manager function

There will be one function which will be started at startup in a separate thread, and will work in an infinite loop. This function will pull out rows from `batchrows` with `status == queued` and call the appropriate `DoSlowQuery()` or `DoBatchJob()` function, process the values returned by that function, update `batches` and `batchjobs`, *etc*, and then rinse and repeat. We refer to this function as the job manager.

This function will operate the following way:
```
type BatchJob_t struct {
    app      string
    op       string
    batch    string
    rowid    int     // unique row ID from batchrows table
    context  structure
    line     integer
    input    structure
}

//
// The initblocks data structure will be updated by concurrent threads, if the job manager
// function is executed in multiple concurrent threads in a single Alya process. It is possible that
// the other three data structures below too may be updated concurrently. Concurrency management
// measures must be implemented.
//
// In the keys to the four hashmaps, "app" is a string, and "app+op" is a concatenation of the "app"
// string and the "op" string.
//
global var initblocks              map[app]InitBlock
global var initfuncs               map[app]Initializer
global var slowqueryprocessorfuncs map[app+op]SlowQueryProcessor
global var batchprocessorfuncs     map[app+op]BatchProcessor

func JobManager()
    var blockofrowsrows []BatchJob_t
    var batchlist       map[string]{}      // implements a set of strings

    while forever do
        blockofrows = []
        batchlist = []

        BEGIN TRANSACTION
        blockofrows = 
                SELECT FOR UPDATE from batches, batchrows (with batches.id == batchrows.batch) WHERE
                        batchrows.status == "queued" LIMIT ALYA_BATCHCHUNK_NROWS
        if blockofrows == empty then
            sleep for a random number of seconds between 30 and 60 secs
            log DEBUG the sleep duration
            continue
        endif
        for each row in blockofrows do
            UPDATE batchrows, set status to "inprog"
        endfor
        if batches.status == "queued" for any of the batches, then
            UPDATE batches.status for that record to "inprog"
        endif
        COMMIT TRANSACTION
        for each thisrow in blockofrows do
            if thisrow.line == -1 then
                //
                // this means we had picked up this row, it's not yet processed, but we tried 
                // calling the initfunc() for this app and the call failed, so we can't process
                // row at least temporarily. We'll try running the initfunc() again in the next
                // iteration. Till then, all the rows which are for this app will need to be
                // skipped.
                //
                continue
            endif
            //
            // get the initobject for this app ready
            //
            if initblocks[thisrow.app] == nil then
                initblocks[thisrow.app] = (initfuncs[thisrow.app]).init()
                if there is an error then
                    initblocks[thisrow.app] = nil
                    for i in blockofrows where blockofrows[i].app == thisrow.app do
                        blockofrows[i].line = -1
                        UPDATE batchrows record for this entry set status to "queued"
                    endfor
                    continue
                endif
            else
                if (initblocks[thisrow.app]).IsValid() == false then
                    (initblocks[thisrow.app]).Close()
                    initblocks[thisrow.app] = (initfuncs[thisrow.app]).init()
                    if there is an error then
                        initblocks[thisrow.app] = nil
                        for i in blockofrows where blockofrows[i].app == thisrow.app do
                            blockofrows[i].line = -1
                            UPDATE batchrows record for this entry set status to "queued"
                        endfor
                        continue
                    endif
                endif
            endif

            if thisrow.line == 0 and batch.type == "Q" then
                //
                // we are processing a slow query here, not a batch row
                //
                processorfunc = slowqueryprocessorfuncs[thisrow.app+thisrow.op]
                status, result, messages, outputfiles, err =
                            processorfunc.DoSlowQuery(initblocks[thisrow.app],
                                                      thisrow.context, thisrow.input)
                if err != nil then
                    log the event with CRITICAL priority
                    set batch.status and batchrows.status to "queued"
                    continue
                else if ((status != success) and (status != failed)) then
                    log the event with CRITICAL priority; return
                else
                    UPDATE the row in batches and in batchrows, set status, result, messages, doneat
                    log the row with INFO priority including app, op, reqID, status
                    update the REDIS status record for this job with 100x the CACHEDUR
                    sqdetails = load the details of the slow query
                    err = processorfunc.MarkDone(initblocks[thisrow.app], thisrow.context, thisrow.context,
                                           sqdetails) 
                    if err != nil then
                        log the event with CRITICAL priority
                    endif
                endif
            else
                //
                // we are processing one record of a batch here
                //
                if batch.type != "B" then
                    //
                    // this should never happen -- it can only be an Alya bug
                    //
                    log the details with CRITICAL priority
                    set batch.status and batchrows.status to "queued"
                    continue
                endif
                //
                // do the processing
                //
                processorfunc = batchprocessorfuncs[thisrow.app+thisrow.op]
                //
                // process the row with this processorfunc
                //
                status, result, messages, blobrows, err =
                            processorfunc.DoBatchJob(initblocks[thisrow.app],
                                                     thisrow.context, thisrow.line, thisrow.input)
                if err != nil then
                    log the event with CRITICAL priority
                    set batch.status and batchrows.status to "queued"
                    continue
                else if ((status != success) and (status != failed)) then
                    log the event with CRITICAL priority; continue
                else
                    UPDATE the record in batchrows with status, result, messages, blobrows, doneat
                    log the row with INFO priority including app, op, batchID, line, status
                    add thisrow.batch to the set called batchlist
                endif
            endif
        endfor each thisrow

        //
        // we have finished a set of records at this point. We now check to see
        // if any of the batches we have processed has now been completed due to
        // our processing. In other words, were the records we processed in this
        // the last few records of the batch? If yes, then we have completed the
        // batch. If yes, then we need to summarise and close the batch as a whole.
        //

        for each onebatch in batchlist do
            begin transaction
            SELECT FOR UPDATE from batches where batches.id == onebatch, fetch the "doneat" field
            if doneat is not NULL then
                //
                // some other thread is summarising or has summarised this
                // batch, I'll quietly exit
                //
                rollback transaction
                log details at INFO level
                continue
            endif
            //
            // I may have to summarise, if the batch is complete
            //
            SELECT FOR UPDATE all records from batchrows where batchrows.batch == onebatch
                              WHERE status is "queued" or "inprog"
            if count of records > 0 then
                //
                // the batch is not yet complete, some rows are pending, so I'll quietly unlock the locks and exit
                //
                rollback transaction
                log details at INFO level
                continue
            endif
            //
            // the batch is complete, no records pending, and the batch has not been summarised,
            // so I will have to do it, else no one else will
            //
            for each onerow in (SELECT FOR UPDATE blobrows from all the batchrows records for this batch, sorted by "line") do
                if the onerow.blobrows names a new logical file which has not been created then
                    create that file in tmp storage
                endif
                append the blobrows strings to the appropriate files
            endfor
            for each logical file thus created do
                move the tmp file to an object in the object storefunc Close(Initblock) (error)
            endfor
            update the "outputfiles" field in the batches record with a JSON block containing the list of object IDs
            update the "nsuccess", "nfailed", "naborted" counts in the batches record for onebatch
            update batches record for onebatch, "status" to "success" or "failed", "doneat" to current timestamp
            commit transaction
            update the status in the REDIS record for this batch with expiry time 100x of usual cache duration

            batchdetails = load the details of the batch
            err = processorfunc.MarkDone(initblocks[thisrow.app], thisrow.context, thisrow.context, batchdetails) 
            if err != nil then
                log the event with CRITICAL priority
            endif
        endfor each onebatch in batchlist[]

        // for each initobj in values_of(initblocks[]) do
        //    initobj.close()
        // done
        //   We don't close the initblock here -- we keep it alive, and we
        //   test its health with an IsAlive() at the start of the next round
        //
        // initblocks = []    // initialise to an empty map
        blockofrows = []
        batchlist   = []

        if there were no records processed in the last iteration
            sleep for a random number of seconds, random value between X and Y secs
        endif
    endwhileforever
end func
```

Therefore, this function will be called every time a message is received from the work queue of the message bus, and will get either a single slow query processed, or a chunk of a batch processed. If it is a batch chunk, it will also summarise the batch and consolidate the output files into aggregate output files.