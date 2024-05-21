# Multi-lingual label standard

In Alya, there will be a master file which will contain
* a string ID
* a hashmap of `(langcode, string)` key-value pairs

From this will be generated language-specific files using a tool. Each language-specific file will contain just the string ID and its corresponding string in that language.

The master file format will be

``` json
{
    "1": {
        "en-IN": "Invalid name",
        "hi-IN": "(something in Hindi)"
    },
    "2": {
        "en-IN": "Invalid address",
        "hi-IN": "(something in Hindi)"
    }
}
```

The language-specific file will have the format
``` json
{
    "1": "Invalid name",
    "2": "Invalid address"
}
```

The name of the language-specific file will be of the form `XYZ-en-IN.json`, where the `XYZ` will depend on the application. All the language-specific files for a given prefix of `XYZ` will have the same filename format, and will carry the same set of keys, with strings in different formats. But an application may have an `XYZ*` set of language-specific files, another `PQR*` set of files, and so on. They will be generated from their respective master files. The keys within a single file will have to be unique, and the set of keys in all the language variants of a set will need to be consistent and uniform -- it is not acceptable to have a key missing for the Hindi file but present in the Spanish file.