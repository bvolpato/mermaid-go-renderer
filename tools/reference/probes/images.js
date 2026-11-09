// Image probe: the rectangles of the HTML in a foreignObject before and
// after its images load.
//
// The script is the body of an async function for `oracle.mjs eval`. Each
// case of `args.cases` is `{ name, html }`. The page puts the markup into
// div#container and reads getBoundingClientRect of every element in #fo two
// times: at once, while each image is still pending, and after each image
// fired its load or error event (or is complete with no event, as an image
// with no src is). `natural` has naturalWidth and naturalHeight then.
const container = document.getElementById('container');
const out = [];
const rects = () =>
  [...container.querySelectorAll('#fo *')].map((el) => {
    const b = el.getBoundingClientRect();
    return [b.x, b.y, b.width, b.height];
  });
for (const c of args.cases) {
  container.innerHTML = c.html;
  const imgs = [...container.querySelectorAll('img')];
  const before = rects();
  const events = await Promise.all(
    imgs.map(
      (img) =>
        new Promise((res) => {
          if (img.complete) {
            res('complete');
            return;
          }
          img.addEventListener('load', () => res('load'));
          img.addEventListener('error', () => res('error'));
        })
    )
  );
  out.push({ name: c.name, events, natural: imgs.map((i) => [i.naturalWidth, i.naturalHeight]), before, after: rects() });
}
container.innerHTML = '';
return out;
