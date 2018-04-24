import { LogLogPlot } from "./loglogplot";

declare var d3: any;

export interface IFollowers {
    country: string;
    count: number;
    followers: number;
}

export class FollowerDistributionPlot extends LogLogPlot {
    constructor(public element: HTMLElement, public data: IFollowers[], countryNames = ["India", "China"]) {
        super(element, "# of Followers", "# of Accounts");
        const line = d3.line()
            .x((d: IFollowers) => this.xScale(d.followers + 1))
            .y((d: IFollowers) => this.yScale(d.count));

        const cumsum = (d: IFollowers[]) => {
            let previous = 0;
            return d.map((x) => { x.count += previous; previous = x.count; return x; });
        };
        const countries = countryNames.map((name) => cumsum(data.filter((d) => d.country === name)));

        const nodes = this.svg.selectAll(".country").data(countries)
            .enter()
            .append("g")
            .attr("class", "country")
            .attr("data-country-name", (d: IFollowers[]) => d[0].country);

        nodes.append("path")
                .style("fill", "none")
                .style("stroke", (_: any, i: number) =>  d3.schemeCategory10[i])
                .attr("d", (country: IFollowers[]) => line(country));

        nodes.append("text")
            .text((country: IFollowers[]) => country[0].country)
            .style("font-size", "12px")
            .attr("x", (country: IFollowers[]) => this.xScale(country[0].followers + (this.logXScale ? 1 : 0)))
            .attr("y", (country: IFollowers[]) => this.yScale(country[0].count));
    }

    public resetScales(): void {
        super.resetScales();
        if (this.logXScale) {
            this.xAxis.tickFormat((x: number) => "≥ " + (x - 1).toLocaleString());
            this.xScale.domain([1, 100000]);
        } else {
            this.xAxis.tickFormat((x: number) => "≥ " + (x).toLocaleString());
            this.xScale.domain([0, 100000]);
        }
        this.yScale.domain([1, 500000]);
    }

    public redraw(transition = false): void {
        super.redraw(transition);
        let selection = this.svg;
        if (transition) {
            selection = this.svg.transition("countries").duration(1500);
        }
        const line = d3.line()
            .x((d: IFollowers) => this.xScale(d.followers + 1))
            .y((d: IFollowers) => this.yScale(d.count));

        selection.selectAll(".country path")
            .attr("d", (country: IFollowers[]) => line(country));

        selection.selectAll(".country text")
            .attr("x", (country: IFollowers[]) => this.xScale(country[0].followers + 1))
            .attr("y", (country: IFollowers[]) => this.yScale(country[0].count));
    }
}
