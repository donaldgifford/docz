/*
 * The compiled binary's entry (DESIGN-0024 section 10): main with the
 * page's assets embedded by scripts/embed-manifest.ts, so the binary
 * needs no dist-review beside it. A checkout runs review/main.ts, which
 * serves dist-review from disk instead.
 */
import { ASSETS } from "./__generated__/assets";
import { main, processIo } from "./main";
import { mapAssets } from "./server";

process.exitCode = await main(processIo(mapAssets(ASSETS)));
