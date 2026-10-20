A. The SERVICE section
[SERVICE]
    Flush        1
    Log_Level    info

Flush 1 tells Fluent Bit to flush its accumulated output approximately every second.
This is a batching and latency setting, not a guarantee that every record reaches its destination within one second.
Log_Level info controls the verbosity of Fluent Bit's own operational logs.
Remember that Fluent Bit's own logs are different from the application logs it's collecting.
For example:
- Application log: database transaction failed
- Fluent Bit log: input initialized successfully
When debugging our pipeline, we'll need to distinguish between the two.
B. The INPUT section
[INPUT]
    Name              tail
    Path              /var/log/app/app.log
    Tag               go.app
    Parser            json
    Read_from_Head    true
    DB                /tmp/fluent-bit-tail.db

This is the most important section for today's experiment.
Name tail
Selects Fluent Bit's file-tailing input plugin.
Path /var/log/app/app.log
Specifies the file Fluent Bit reads.
Notice that this is a path inside the Fluent Bit container, not the path on your Ubuntu machine. We'll connect the two paths through a Docker volume.
Tag go.app
Assigns the tag go.app to the collected records.
Tags help Fluent Bit route records to appropriate outputs. Later, we could send application logs to Elasticsearch while routing different logs elsewhere.
Parser json
Tells Fluent Bit to interpret each line as JSON.
Our Go application already generates JSON, so we don't need to invent a custom log format.
Read_from_Head true
Tells Fluent Bit to read existing file contents from the beginning when it first discovers the file, rather than starting only with new lines.
This is important because our application has already written two log events.
DB /tmp/fluent-bit-tail.db
Fluent Bit maintains a database of file-tracking information, including offsets.
Conceptually, it needs to know how far it has progressed through the file.
Without persistent tracking state, restarting a collector can cause it to reread existing logs or miss expected data, depending on its configuration and how files change.
For this initial experiment, the database is in the container's temporary filesystem. We'll deliberately change that later to investigate restart behavior.
C. The OUTPUT section
[OUTPUT]
    Name    stdout
    Match   go.app
    Format  json_lines

We're using the stdout output plugin.
It prints collected records to Fluent Bit's standard output, allowing us to inspect them using Docker logs.
Match go.app means this output accepts records with the go.app tag.
Format json_lines makes the output easier to inspect because each record is printed as a JSON line.
Why not send the logs directly to Elasticsearch now?
Because we want to isolate the collector first.
If records don't appear, we can investigate the input, file access, parsing, and routing without also debugging Elasticsearch connectivity or indexing.
