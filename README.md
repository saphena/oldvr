# OLD VOICE RECORDINGS

This provides standalone access to a static collection of voice recordings from the now defunct
BT voice recorder originally in Runcton.

I am my own web server accessing my own SQLite database and voice recordings stored in MP3 format
on disk. Recordings themselves are separate files in \[a single\] disk folder but they're indexed
by date and phone number in the database.

This is not intended to be a changeable dataset.

## Database contents

### Table **cdrs**
This contains one record for each Call Detail Record as follows:-
|   Field   |  Type    | Contents         |
|-----------|----------|------------------|
| cdrid     | string   | unique reference |
| direction | boolean  | in/out           |
| duration  | timespan | hh:mm:ss         |
| connected | datetime | ISO8601          |
| aphone    | string   | calling number   |
| bphone    | string   | called number    |
| folderid  | integer  | data path ix     |
| dvdname*  | string   | archive name     |

These records are extracted and reformatted as necessary from original BT database.

\* dvdname is the label of the DVD which should also contain this record. DVDs are not actually used
by this system at all.

### Table **folders**
This contains one record for each data volume folder managed by the system.
|   Field   |  Type    | Contents         |
|-----------|----------|------------------|
| folderid  | integer  | unique reference |
| datapath  | string   | full path        |

### Table **params**
This contains one record.
|   Field   |  Type    | Contents                     |
|-----------|----------|------------------------------|
| dbname    | string   | description of this database |
