"""Bounded-output local execution; no shell or third-party process dependency."""
import os
import signal
import subprocess
import sys
import threading
import time

class Buffer:
    def __init__(self, limit):
        self.limit, self.total, self.data = limit, 0, bytearray()
    def drain(self, stream):
        try:
            while chunk := stream.read(8192):
                self.total += len(chunk)
                self.data.extend(chunk[:max(0, self.limit - len(self.data))])
        finally:
            stream.close()

def stop(process):
    if os.name == "posix":
        try:
            os.killpg(process.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
    elif process.poll() is None:
        process.kill()
    process.wait()

def run(command, cwd, timeout, output_limit):
    resolved = [sys.executable if arg == "{python}" else arg for arg in command]
    start = time.monotonic()
    try:
        process = subprocess.Popen(resolved, cwd=cwd, stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=os.name == "posix")
    except OSError as error:
        return {"status": "error", "exit_code": None, "error": f"launch_error:{error.errno}",
                "duration_seconds": round(time.monotonic() - start, 6), "stdout": "", "stderr": "",
                "output_bytes": 0, "output_truncated": False}
    buffers = [Buffer(output_limit), Buffer(output_limit)]
    threads = [threading.Thread(target=b.drain, args=(s,), daemon=True)
               for b, s in zip(buffers, (process.stdout, process.stderr))]
    for thread in threads:
        thread.start()
    status = "passed"
    try:
        code = process.wait(timeout=timeout)
        if code != 0:
            status = "failed"
    except subprocess.TimeoutExpired:
        status = "timeout"
        stop(process)
        code = process.returncode
    except KeyboardInterrupt:
        status = "cancelled"
        stop(process)
        code = process.returncode
    # A child retaining a pipe must not hang the runner after its parent exits.
    for thread in threads:
        thread.join(timeout=0.2)
    if any(thread.is_alive() for thread in threads):
        if os.name == "posix":
            stop(process)
            for thread in threads:
                thread.join(timeout=0.2)
        if any(thread.is_alive() for thread in threads):
            status = "error"
    return {"status": status, "exit_code": code, "duration_seconds": round(time.monotonic() - start, 6),
            "stdout": bytes(buffers[0].data).decode("utf-8", errors="replace"),
            "stderr": bytes(buffers[1].data).decode("utf-8", errors="replace"),
            "output_bytes": sum(b.total for b in buffers),
            "output_truncated": any(b.total > b.limit for b in buffers)}
