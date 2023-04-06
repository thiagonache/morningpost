# The Morning Post

It's nice to have something interesting to read with your morning coffee. My task with this project is to write a Go program that will curate a little “morning newspaper” for you.


[Demo Video](https://thiagocarvalho-public-assets.s3.us-west-2.amazonaws.com/morningpost-videos/demo.gif)

## Getting started

### Install binary

You can `install` via Go command:

```bash
$ go install github.com/thiagonache/morningpost/cmd/morningpost@latest
$
```

Alternatively, you can `download` the compiled binary from the [releases](https://github.com/thiagonache/morningpost/releases) page.

### Run

Run program and open your browser at <http://localhost:33000/>

```bash
$ morningpost
2023/04/03 10:22:11 Listening at http://0.0.0.0:33000
```

## Documentation

Visit our [documentation page](doc/README.md)

## RoadMap

- [x] ~~CLI~~
- [x] Add Feed type `RSS`
- [x] Web UI
- [x] Add Feed type `Atom`
- [x] Add Feed type `RDF`
- [x] Add Feed by `Auto-Discovery`
- [x] Implement HTMX
- [x] Delete Feed
- [x] Infinite scroll for News
- [ ] Add `custom` Feed via Go `interface`
- [ ] Decouple store
- [ ] Improve test coverage
- [ ] Recommend new Feeds based on feeds already added
- [ ] Recommend news based on previous News read
- [ ] Do Auto-Discovery via [HTMX Active Search](https://htmx.org/examples/active-search/)

## Acknowledgements

Many thanks to [Josh Akeman](https://github.com/joshakeman) for suggesting this project.
