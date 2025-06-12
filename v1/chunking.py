import json
import sys

from chonkie import RecursiveChunker

EOL = "<EOL>"
READY = "<READY>"


class Chunking:
    def __init__(self):
        self.chunker = RecursiveChunker()

    def repr(self):
        lines = []
        print(READY, file=sys.stdout)

        for line in sys.stdin:
            line = line.rstrip()
            if line != EOL:
                lines.append(line)
                continue

            chunks = self.chunker("\n".join(lines))
            lines = []
            for chunk in chunks:
                json.dump({
                    "token_count": chunk.token_count,
                    "text": chunk.text,
                }, sys.stdout)
                print("", file=sys.stdout)
            print(EOL, file=sys.stdout)
