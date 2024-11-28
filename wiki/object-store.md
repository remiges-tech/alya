This is a layer on top of any popular open source object store (we use MinIO for our projects). This module allows our business applications to store, track, process and retrieve objects from the object store. The module treats an object as a typeless array of bytes.

It is assumed that the underlying object-store service will allow our framework to attach an arbitrary JSON block as metadata, replace this metadata block, *etc*.

## Required attributes

When this module creates a new object, the following attributes are part of the `new()` call:
* `blob`: the bag of bytes, either base64-encoded or raw binary
* `is_encoded`: boolean, indicating whether `blob` is base64 encoded or not. The function will know what to do with `blob` based on this value.
* `bucket`: mandatory, string, the name of a bucket. The application administrator will have to create the buckets which the code will use
* `compress`: boolean. If `true`, it tells the module that this blob's data must be compressed and then stored. Since compression is always done first, before any encoding, this means that if `is_encoded` and `compress` are both `true`, the `new()` function will first decode the blob, then compress it, then proceed with whatever else is needed

## Suggested `info` attributes

The object store module does not look at these attributes. They are attached with the object by passing an optional JSON block, called `info`. The attributes in `info` could be:
* `mimetype`: string, something like `text/html` or `application/postscript`
* `ownerclass`: string, indicates the type of entity with which this object is to be attached. For instance, this blob may be getting associated with an accounting voucher (`ownerclass=acctvoucher`) or an MIS report (`ownerclass=annualmis`) and so on
* `ownerid`: string, indicating some (hopefully) unique ID which helps the caller application to identify exactly which voucher or MIS report this blob is supposed to be associated with
* `filename`: string, giving a real-world filename from where this blob came, if the application needs to preserve this information
* `from`: string, containing anything which may help the application identify where the blob came from

## Functions in the library

### `ObjectNew()`

**Request**

```
func Alya.ObjectNew(blob, bucket, info string, isEncoded, compress bool) (id string, nbytes int, err error)
```
where the parameters are as discussed above.

**Processing**

The function will compress the blob if `compress==true`. If `isEncoded` is also `true`, then it will first decode, then note the decoded size in bytes, and then compress. It will then store the (perhaps compressed) blob into the store, and return the unique ID the store returns. It will associate the following meta-data with the object in the store:
* the size in bytes
* the timestamp when the blob is being stored
* a boolean indicating whether the data is compressed or not by the function
* the contents of the `info` JSON blob as-is

**Response**

The response will specify:
* `id`: a unique string identifying this blob
* `nbytes`: an integer specifying the uncompressed size of the blob, if the request asked for compression, or the as-is size if no compression was asked for
* `err`: as usual

### `ObjectInfoUpdate()`

This call will replace any earlier `info` block with a new block. If the new block is an empty JSON block, then the empty JSON block will be stored.

### `ObjectMove()`

This call will take an ID, a source bucket name, and a destination bucket name, and will move the object from one bucket to another. It will first check that the source bucket correctly specifies where the object is currently stored.

### `ObjectInfo()`

This call will take an object ID and return the full contents of the `info` JSON block, plus other variables.

**Request**
```
func Alya.ObjectInfo(id, bucket string) (info string, size integer, isCompressed bool, err error)
```

**Response**

The response will contain
* `info`: a JSON block with metadata which was passed to the object in `ObjectNew()` or `ObjectInfoUpdate()`
* `size`: the number of bytes the object will occupy if uncompressed
* `isCompressed`: boolean, indicating whether the object store module is storing the data in compressed form or raw form


### `ObjectGet()`

This call will take an ID and a bucket name and return the object in uncompressed form. This means that if `compress` was set to `true` in the `ObjectNew()` call, then the function will pull out the blob from secondary storage, uncompress it and then return it.

### `ObjectDelete()`

This call will take an ID and a bucket name and delete the object.