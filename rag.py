import os.path

import fire
import json
import sys

from chonkie import RecursiveChunker


class RAG:
    def chunking(self, path: str, output: str = "-", recipe_path: str = "./chonkie-recipes/default_en.json"):
        chunker = RecursiveChunker.from_recipe(path=recipe_path, return_type="texts")
        with open(path, "r", encoding="utf-8") as f:
            lines = f.readlines()
            chunks = chunker.chunk("\n".join(lines))
            d = {
                "file_name": os.path.basename(path),
                "chunks": [{"index": i, "text": chunks[i].strip()} for i in range(len(chunks))],
            }

            if output == "-":
                json.dump(d, sys.stdout)
                return

            with open(output, "w", encoding="utf-8") as out_file:
                json.dump(d, out_file)


if __name__ == "__main__":
    fire.Fire(RAG)
