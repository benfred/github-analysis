
declare var d3: any;
declare var topojson: any;
declare var mapdata: any;

export class WorldMap {
    public width: number;
    public height: number;

    public scaling: number = 1;
    public strokeWidth: number = 1;
    public previousZoom: string = null;

    public projection = d3.geoMercator();
    public path = d3.geoPath();

    // TODO: These are all d3 selections ... don't define as any
    public chart: any;
    public svg: any;
    public outer: any;
    public mapGroup: any;

    public offsetLeft: number;
    public offsetTop: number;

    protected fullSizeCountries = {
        "Australia": 1, "Canada": 1, "China": 1, "France": 1,
        "India": 1, "Russia": 1, "United States": 1,
    };

    constructor(public element: HTMLElement) {
        this.chart = d3.select(this.element);
        this.width = this.element.offsetWidth;
        this.height = this.width * .5;
        this.offsetLeft = this.element.offsetLeft + 10;
        this.offsetTop = this.element.offsetTop + 10;

        this.chart.selectAll("svg").data([0]).enter().append("svg");

        this.svg = this.chart.select("svg")
            .attr("width", this.width)
            .attr("height", this.height);

        this.projection
            .scale(this.width / 6)
            .translate([this.width * .48, this.height / 1.7]);

        this.path.projection(this.projection);

        this.outer = this.svg.append("g");
        this.mapGroup = this.outer.append("g");

        this.mapGroup.append("g")
            .attr("class", "boundary")
            .selectAll("boundary")
            .attr("stroke-width", this.strokeWidth / this.scaling)
            .data(topojson.feature(mapdata, mapdata.objects.countries).features)
            .enter().append("path")
            .attr("name", (d: any) => d.properties.name)
            .attr("id", (d: any) => d.properties.name)
            .on("click", (d: any) => this.zoomCountry(d.properties.name))
            .attr("d", this.path);
        window.addEventListener("resize", () => this.resize());
    }

    public transitionMap(x: number, y: number, s: number, country: any = null): void {
        this.chart.selectAll("path").attr("filter", null);
        this.chart.selectAll("feGaussianBlur").attr("stdDeviation", 5 / s);

        // update global state.
        this.scaling = s;
        const t = this.outer.transition().duration(1000);
        t.attr("transform", `translate(${x}, ${y}) scale(${this.scaling})`);
        t.select(".boundary")
            .style("stroke-width", (this.strokeWidth / this.scaling) + "px");

        if (country) {
            this.highlightCountry(t, country);
        }
    }

    public highlightCountry(_: any, current: any): void {
        current.classed("selected", true);
    }

    public removeCountryHighlight(): void {
        this.chart.select(".selected").classed("selected", false);
    }

    public zoomCountry(countryName: string): void {
        this.removeCountryHighlight();
        const current = this.chart.select(`[id="${countryName}"]`);
        if (current.nodes().length && (countryName !== this.previousZoom)) {
            this.chart.select(".countrylabel").text(countryName);
            this.chart.select(".close").style("display", "inline-block").on("click", () => this.clearZoom());

            this.previousZoom = countryName;

            // zoom to 50% of the size of the country, unless it looks weird then show full sized
            let zoomRatio = .5;

            if (countryName in this.fullSizeCountries) {
                zoomRatio = 1.0;
            }

            let scaling = 1;
            const bounds = current.nodes()[0].getBBox();
            const widthRatio = this.width / bounds.width;
            const heightRatio = this.height / bounds.height;
            let x;
            let y;
            if (widthRatio > heightRatio) {
                // this isn't quite right in general, but works for zoomRatio of .5 and 1.0
                scaling = heightRatio * zoomRatio;
                x = -(bounds.x - bounds.width * (1 - zoomRatio)) * scaling
                    + (widthRatio - heightRatio) * bounds.width / 2;
                y = -(bounds.y - bounds.height * (1 - zoomRatio)) * scaling;
            } else {
                scaling = widthRatio * zoomRatio;
                x = -(bounds.x - bounds.width * (1 - zoomRatio)) * scaling;
                y = -(bounds.y - bounds.height * (1 - zoomRatio)) * scaling
                    + (heightRatio - widthRatio) * bounds.height / 2;
            }
            this.transitionMap(x, y, scaling, current);
        } else {
            this.clearZoom();
        }
    }

    public clearZoom(): void {
        this.chart.select(".countrylabel").text("");
        this.chart.select(".close").style("display", "none");
        this.removeCountryHighlight();
        this.transitionMap(0, 0, 1);
        this.previousZoom = null;
    }

    public resize(): void {
        this.width = this.element.offsetWidth;
        this.height = this.width * .5;
        this.svg.attr("width", this.width)
            .attr("height", this.height);
        this.projection
            .scale(this.width / 6)
            .translate([this.width * .48, this.height / 1.7]);
        this.path.projection(this.projection);
        this.mapGroup.selectAll("path")
            .attr("d", this.path);

        if (this.previousZoom !== null) {
            // hack: if we're zoomed in on screen size change, rezoom
            const current = this.previousZoom;
            this.previousZoom = null;
            this.zoomCountry(current);
        }
        this.offsetLeft = this.element.offsetLeft + 10;
        this.offsetTop = this.element.offsetTop + 10;
    }
}
