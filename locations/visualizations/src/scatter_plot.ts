declare var d3: any;

import { linear as linearRegression } from "regression";

import { countryEmoji } from "./country_emoji";
import { ILocation } from "./location";
import { LogLogPlot } from "./loglogplot";

export class ScatterPlot extends LogLogPlot {
    public tooltip: any;
    public loglogRegression = true;

    constructor(public element: HTMLElement, public data: ILocation[],
                public x = (c: ILocation) => c.population,
                public y = (c: ILocation) => c.count,
                public xLabel = "Population") {
        super(element, xLabel, "GitHub Accounts");
        this.data = data = data.filter((l) => (x(l) > 0));

        // annotate dots with country flags for the upper/lower pareto frontier
        const display: { [id: string]: number } = {};
        this.data.sort((a, b) => x(a) - x(b));
        const paretoFrontier: ILocation[] = [];
        let previousY = 0;
        data.forEach((c) => {
            const current = y(c);
            if (current > previousY) {
                previousY = current;
                paretoFrontier.push(c);
                display[c.country] = 1;
            }
        });

        // lower pareto frontier
        this.data.sort((a, b) => x(b) - x(a));
        previousY = Infinity;
        data.forEach((c) => {
            const current = y(c);
            if (current < previousY) {
                previousY = current;
                display[c.country] = 1;
            }
        });
        this.tooltip = this.chart.append("div")
            .attr("class", "tooltip hidden");

        this.resetScales();

        // draw trendline
        const points = this.calculateTrendline();
        this.svg.append("marker")
            .attr("id", "arrowmarker")
            .attr("fill", "orange")
            .attr("fill-opacity", .5)
            .attr("viewBox", "0 0 10 10")
            .attr("refX", 0)
            .attr("refY", 5)
            .attr("markerUnits", "strokeWidth")
            .attr("markerWidth", 5)
            .attr("markerHeight", 5)
            .attr("orient", "auto")
            .append("path")
                .attr("d", "M 0 0 L 10 5 L 0 10 z");

        this.svg.selectAll(".trendline")
            .data([points]).enter()
            .append("path")
            .attr("marker-end", "url(#arrowmarker)")
            .attr("class", "trendline")
            .attr("fill-opacity", 0)
            .attr("stroke", "orange")
            .attr("stroke-opacity", .5)
            .attr("stroke-width", this.pointRadius)
            .attr("d", (path: number[][]) => {
                return d3.line()
                    .x((p: number[]) => this.xScale(p[0]))
                    .y((p: number[]) => this.yScale(p[1]))(path);
            });

        const nodes = this.svg.selectAll("g .datapoint")
            .data(this.data)
            .enter()
            .append("g").attr("class", "datapoint");

        const X = (c: ILocation) => this.xScale(this.x(c));
        const Y = (c: ILocation) => this.yScale(this.y(c));

        nodes.append("circle")
            .attr("r", this.pointRadius)
            .attr("fill", "rgb(13, 140, 243)")
            .attr("fill-opacity", .5)
            .attr("cx", X)
            .attr("cy", Y);

        nodes.append("text")
            .attr("font-size", "16px")
            .attr("text-anchor", "middle")
            .attr("data-country", (d: ILocation) => d.country)
            .text((d: ILocation) => d.country in display ? countryEmoji[d.country] : "")
            .attr("x", X)
            .attr("y", Y);

        nodes.on("mouseout", () => this.tooltip.classed("hidden", true))
            .on("mousemove", (d: ILocation) => {
                const mouse = d3.mouse(this.svg.node()).map((t: number) => +t);
                const html = `<i style='color:rgb(102, 102, 102)' class='fa fa-map-marker'></i><b> ${d.country}</b>
                             <br>GitHub Accounts: ${d.count.toLocaleString()}<br>${xLabel}: ${this.xTickFormat(x(d))}`;
                this.tooltip.classed("hidden", false)
                    .attr("style", `left:${mouse[0] + this.element.offsetLeft + 10}px;
                                    top:${mouse[1] + this.element.offsetLeft + 10}px`)
                    .html(html);
            });

        this.chart.select(".loglogr").on("click", () => {
            this.setRegression(true);
        });

        this.chart.select(".linearr").on("click", () => {
            this.setRegression(false);
        });
    }

    public setRegression(loglog: boolean): void {
        this.loglogRegression = loglog;
        const points = this.calculateTrendline();
        this.svg.selectAll(".trendline").data([points]);
        this.redraw(true);
    }

    public resetScales(): void {
        super.resetScales();
        this.xScale.domain([100000, 1400000000]);
        this.yScale.domain([620000, 5]);
        this.xAxis.tickFormat((d: number) => this.xTickFormat(d));
    }

    public xTickFormat(t: number): string {
        return (t / 1000000).toLocaleString() + (this.element.offsetWidth > 500 ? " Million" : "M");
    }

    public redraw(transition = false): void {
        super.redraw(transition);
        let selection = this.svg;
        if (transition) {
            selection = this.svg.transition("points").duration(1500);
        }
        const X = (c: ILocation) => this.xScale(this.x(c));
        const Y = (c: ILocation) => this.yScale(this.y(c));

        selection.selectAll(".datapoint circle")
            .attr("cx", X)
            .attr("cy", Y);
        selection.selectAll(".datapoint text")
            .attr("x", X)
            .attr("y", Y);

        selection.selectAll(".trendline").attr("d", (points: number[][]) => {
            if (this.loglogRegression && this.logXScale && this.logYScale) {
             // fucks up tweening   points = [points[0], points[points.length - 1]];
            }
            return d3.line().x((p: number[]) => this.xScale(p[0]))
                            .y((p: number[]) => this.yScale(p[1]))(points);
        });
    }

    protected calculateTrendline(): number[][] {
        let points = this.data.map((d) => [this.x(d), this.y(d)]);
        if (this.loglogRegression) {
            points = points.map((p: number[]) => p.map(Math.log));
        }

        let result: any;
        if (!this.loglogRegression || ((this.xLabel === "Population"))) {
            // hack: regress on (y, x) instead of (x, y) for population graph
            const swapPoints = (v: number[][]) => v.map((z) => [z[1], z[0]]);
            result = linearRegression(swapPoints(points));
            points = swapPoints(result.points);
        } else {
            result = linearRegression(points);
            points = result.points;
        }
        window.console.log(`${this.xLabel} r2 = ${result.r2} type=${this.loglogRegression ? "Log-Log" : "Linear "}`);

        if (this.loglogRegression) {
            points = points.map((p: number[]) => p.map(Math.exp));
        }

        // points = points.filter((p: number[]) => p[0] > this.xMin);
        points.sort((a, b) => a[1] - b[1]);
        return points;
    }
}
