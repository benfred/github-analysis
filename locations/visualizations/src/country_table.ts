declare var d3: any;

import { countryEmoji } from "./country_emoji";
import { ILocation, Rank, rankLabels } from "./location";

export class CountryTable {
    public filterCount = 10;
    public rank: Rank = "count";
    public chart: any;
    public onChange: (r: Rank) => void = null;

    constructor(element: HTMLElement, public data: ILocation[]) {
        this.chart = d3.select(element);
        this.update();

        this.chart.select(".accounts").on("click", () => this.changeRank("count"));
        this.chart.select(".logfollowers").on("click", () => this.changeRank("logfollowers"));
        this.chart.select(".followers").on("click", () => this.changeRank("followers"));
        this.chart.select(".population").on("click", () => this.changeRank("accountsper1M"));
        this.chart.select(".gdp").on("click", () => this.changeRank("accountsper1Bgdp"));
        this.chart.select(".more").on("click", () => {
            if (this.filterCount <= 10) {
                this.filterCount = 30;
                this.chart.select(".more").text("less ...");
            } else {
                this.filterCount = 10;
                this.chart.select(".more").text("more ...");
            }
            this.update();
        });
    }

    public update(): void {
        const body = this.chart.select("tbody");
        body.selectAll("tr").data([]).exit().remove();

        this.data.sort((a, b) => b[this.rank] - a[this.rank]);
        const current = this.filterData();
        const max = this.data[0][this.rank];

        const rows = body.selectAll("tr").data(current)
            .enter()
            .append("tr");

        rows.append("td").html((l: ILocation) => this.formatLocation(l));

        rows.append("td").style("text-align", "center")
            .text((d: ILocation) => d[this.rank].toLocaleString());

        // elementary bar chart
        rows.append("td")
            .style("text-align", "center")
            .append("div")
            .style("background-color", "steelblue")
            .style("width", (d: ILocation) => (100 * d[this.rank] / max) + "%")
            .html("&nbsp");
    }

    public changeRank(r: Rank, propagate = true) {
        this.rank = r;
        this.chart.select(".rank_label").text(rankLabels[r]);
        this.update();
        if (this.onChange && propagate) {
            this.onChange(r);
        }
    }

    public filterData(): ILocation[] {
        return this.data
            .filter((x) => x.population > 100000)
            .slice(0, this.filterCount);
    }

    public formatLocation(l: ILocation): string {
        return (countryEmoji[l.country] || "") + " " + l.country;
    }
}
