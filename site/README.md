# clowd.jooo.sh

The landing page and docs for Clowd. Next.js (App Router, static export),
Tailwind CSS v4, MDX for docs, `motion` for animation and `lucide-react` for
icons.

```sh
npm install
npm run dev      # http://localhost:3000
npm run build    # static site in out/
```

## Layout

```
src/app/page.tsx          landing page
src/app/docs/*/page.mdx   docs, one folder per page
src/lib/docs.ts           docs sidebar order
src/lib/clowd.ts          Clowd syntax highlighter; its colours are the site palette
src/lib/demo.ts           the hero build animation and its measured numbers
src/lib/toy.ts            the "did you mean" toy checker
src/data/app.aws.tf.json  real `clowd build --target aws` output of examples/app.clowd
```

## Keeping it true

Everything the page shows was taken from the compiler, and should be
refreshed when the compiler changes:

- `src/data/app.aws.tf.json`: copy `.clowd/build/aws/main.tf.json` after
  `clowd build --target aws` on `examples/app.clowd`, then update
  `JSON_LINES` and the per-statement resource lists in `src/lib/demo.ts`
  (every resource must appear exactly once).
- `CHECK_MS` in `src/lib/demo.ts`: the median of a few runs of
  `clowd check examples/app.clowd`.
- `src/lib/toy.ts` copies `clowd check`'s messages for wrong relationships;
  `VERB_GUESSES` in `src/lib/clowd.ts` mirrors
  `internal/syntax/parser/words.go`.
- The matrix and terminal output in `src/components/` is real `clowd check`
  output.

## Deploying

`.github/workflows/pages.yml` builds and deploys on every push to `main`
that touches `site/`. `public/CNAME` sets the custom domain.
