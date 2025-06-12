import json
import sys

from chonkie import RecursiveChunker

EOL = "<EOL>"
READY = "<READY>"
EXIT = "<EXIT>"


class Chunking:
    def __init__(self):
        self.chunker = RecursiveChunker()

    def repr(self):
        lines = []
        print(READY, file=sys.stdout, flush=True)

        for line in sys.stdin:
            line = line.rstrip()
            if line == EXIT:
                break

            if line != EOL:
                lines.append(line)
                continue

            chunks = self.chunker("\n".join(lines))
            lines = []
            for chunk in chunks:
                d = {
                    "token_count": chunk.token_count,
                    "text": chunk.text,
                }
                json.dump(d, sys.stdout)
                print("", file=sys.stdout)
                sys.stdout.flush()
            print(EOL, file=sys.stdout, flush=True)
