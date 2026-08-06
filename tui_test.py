#!/usr/bin/env python3
"""Drive prts TUI through pty with a timestamped transcript."""
import json, os, pty, select, struct, sys, termios, fcntl, time

def main(argv):
    actions = json.load(open('/tmp/tui_actions.json'))
    pid, fd = pty.fork()
    if pid == 0:
        os.execvp(argv[0], argv)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack('HHHH', 40, 100, 0, 0))
    transcript = []
    start = time.time()
    idx = 0
    last_len = 0
    while time.time() - start < 45:
        r, _, _ = select.select([fd], [], [], 0.15)
        if r:
            try:
                data = os.read(fd, 65536)
            except OSError:
                break
            if not data:
                break
            # Respond to termenv's OSC 11 background query so it never stalls.
            if b'\x1b]11;?\x1b\\' in data:
                os.write(fd, b'\x1b]11;rgb:0b0e/1420/1410\x1b\\')
            transcript.append((round(time.time()-start,2), data.decode('utf-8','replace')))
        elapsed = time.time() - start
        while idx < len(actions) and elapsed >= actions[idx]["time"]:
            os.write(fd, actions[idx]["keys"].encode())
            transcript.append((round(time.time()-start,2), '>>>SEND ' + repr(actions[idx]["keys"])))
            idx += 1
    try:
        os.close(fd)
    except OSError:
        pass
    try:
        os.waitpid(pid, 0)
    except ChildProcessError:
        pass
    with open('/tmp/tui_transcript.txt','w') as f:
        for t, chunk in transcript:
            f.write(f"[{t:6.2f}] {chunk}\n")
    print("done,", idx, "actions,", len(transcript), "chunks")

if __name__ == '__main__':
    main(sys.argv[1:])
