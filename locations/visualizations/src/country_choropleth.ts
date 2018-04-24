import { displayRank, ILocation, Rank, rankLabels } from "./location";
import { WorldMap } from "./worldmap";

declare var d3: any;

export function createDropShadowFilter(svg: any) {
    const defs = svg.selectAll("defs").data([0]).enter().append("defs");

    const filter = defs.append("filter")
        .attr("id", "dropshadow");

    filter.append("feGaussianBlur")
        .attr("in", "SourceAlpha")
        .attr("stdDeviation", 2)
        .attr("result", "blur");

    filter.append("feOffset")
        .attr("in", "blur")
        .attr("dx", 0)
        .attr("dy", 0)
        .attr("result", "offsetBlur");

    const feMerge = filter.append("feMerge");

    feMerge.append("feMergeNode")
        .attr("in", "offsetBlur");

    feMerge.append("feMergeNode")
        .attr("in", "SourceGraphic");
}

export class CountryChoropleth extends WorldMap {
    public color = d3.scaleLinear();
    public countries: { [id: string]: ILocation } = {};
    public ranks: { [id: string]: number } = {};
    public tooltip: any;
    public rank: Rank = "count";
    public onChange: (r: Rank) => void = null;

    constructor(element: HTMLElement, public data: ILocation[]) {
        super(element);
        createDropShadowFilter(this.svg);
        this.color.range(d3.schemeBlues[9]);
        this.prepareData(this.rank);
        data.forEach((country) => this.countries[country.country] = country);

        this.mapGroup.selectAll("path").style("fill",
            (d: any) => this.color(this.ranks[d.properties.name] || 0));

        this.tooltip = this.chart.append("div")
            .attr("class", "tooltip hidden");

        this.mapGroup.selectAll("path")
            .on("mousemove", (d: any) => {
                const country = this.countries[d.properties.name];
                if (!country) {
                    window.console.log("missing", d.properties.name);
                    this.tooltip.classed("hidden", true);
                    return;
                }
                const mouse = d3.mouse(this.svg.node()).map((val: number) => +val);
                this.tooltip.classed("hidden", false)
                    .attr("style", `left:${mouse[0] + this.offsetLeft}px;top:${mouse[1] + this.offsetTop}px`)
                    .html(`<i style='color:rgb(102, 102, 102)' class='fa fa-map-marker'></i><b> ${country.country}</b>
                          <br>${rankLabels[this.rank]}: ${country[this.rank].toLocaleString()}`);
            })
            .on("mouseout", () => this.tooltip.classed("hidden", true));

        this.chart.select(".accounts").on("click", () => this.changeRank("count"));
        this.chart.select(".logfollowers").on("click", () => this.changeRank("logfollowers"));
        this.chart.select(".followers").on("click", () => this.changeRank("followers"));
        this.chart.select(".population").on("click", () => this.changeRank("accountsper1M"));
        this.chart.select(".gdp").on("click", () => this.changeRank("accountsper1Bgdp"));
    }

    public zoomCountry(countryName: string): void {
        const country = this.countries[countryName];

        this.chart.select(".infobox").style("display", null);
        let topMargin = 0;
        if (this.width > 1000) {
            // on large displays, sometimes infobox is off the page. Hack it upwards in a way that looks ok
            topMargin = -this.height / 5;
            if ((countryName !== "France") && (countryName !== "Russia") && (countryName in this.fullSizeCountries)) {
                topMargin = -this.height / 20;
            }
        }
        this.chart.select(".infobox").style("margin-top", topMargin + "px");

        if (country) {
            let gdp = "$" + (country.gdp / 1000000000).toLocaleString() + " Billion" + displayRank(country.ranks.gdp);
            let pop = (country.population / 1000000).toLocaleString() + " Million"
                 + displayRank(country.ranks.population);
            let accountpop = country.accountsper1M.toLocaleString() + displayRank(country.ranks.accountsper1M);
            let accountgdp = country.accountsper1Bgdp.toLocaleString() + displayRank(country.ranks.accountsper1Bgdp);

            if (country.gdp === 0) {
                accountgdp = gdp = "Unknown";
            }
            if (country.population === 0) {
                accountpop =  pop = "Unknown";
            }

            const html = `<table class="table" style="table-layout:fixed;width:100%;max-width:100%;margin-bottom:0px">
<thead><tr><th></th><th></th></tr></thead>
<tbody>
<tr><td>GitHub Accounts</td><td>${country.count.toLocaleString()}${displayRank(country.ranks.count)}</td></tr>
<tr><td>Accounts / 1M Population</td><td>${accountpop}</td></tr>
<tr><td>Accounts / $1B GDP</td><td>${accountgdp}</td></tr>
<tr><td>Total Followers</td><td>${country.followers.toLocaleString()}${displayRank(country.ranks.followers)}</td></tr>
<tr><td>Population</td><td>${pop}</td></tr>
<tr><td>GDP</td><td>${gdp}</td></tr>
</tbody>
<tfoot><tr><th></th><th></th></tr></tfoot>
</table>`;
            // TODO: decide if we want to show this
            // <tr><td>∑ log(followers)</td><td>${country.logfollowers.toLocaleString()}</td></tr>

            this.chart.select(".infobox_body").html(html);

        } else {
            this.chart.select(".infobox_body").html("No data");
        }
        super.zoomCountry(countryName);
    }

    public clearZoom(): void {
        this.chart.select(".infobox").style("display", "None");
        super.clearZoom();
    }

    public highlightCountry(transition: any, current: any): void {
        //  Dropshadow for us/france is slow on firefox, but fast on safari/chrome =()
        const name = current.nodes()[0].id;
        if (navigator.userAgent.toLowerCase().indexOf("firefox") > -1) {
            if ((name === "United States") || (name === "France")) {
                window.console.log(`not highlighting ${name}. dropshadow too slow on firefox =(. TODO: investigate`);
                return;
            }
        }

        // move country to the top and add a dropshadow
        current.nodes().forEach((e: HTMLElement) => e.parentNode.appendChild(e));
        this.chart.selectAll("feGaussianBlur").attr("stdDeviation", 5 / this.scaling);
        transition.on("end", () => { current.attr("filter", "url(#dropshadow)"); });
    }

    public removeCountryHighlight(): void {
        this.chart.selectAll("path").attr("filter", null);
    }

    public prepareData(rank: Rank) {
        this.ranks = {};
        let maxvalue = 0;
        this.data.forEach((d) => {
            const r = Math.sqrt(d[rank]);
            maxvalue = Math.max(maxvalue, r);
            this.ranks[d.country] = r;
        });
        const steps = this.color.range().length;
        const stepSize = maxvalue / (steps - 2);
        this.color.domain(d3.range(-stepSize, maxvalue + 1e-10, stepSize));
    }

    public changeRank(rank: Rank, propagate = true) {
        this.rank = rank;
        this.prepareData(rank);
        this.chart.select(".rank_label").text(rankLabels[rank]);
        this.mapGroup.selectAll("path").transition().duration(750)
            .style("fill", (d: any) => this.color(this.ranks[d.properties.name] || 0));

        if (this.onChange && propagate) {
            this.onChange(rank);
        }
    }
}
