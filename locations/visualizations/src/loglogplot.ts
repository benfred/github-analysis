declare var d3: any;

export class LogLogPlot {
    public width: number;
    public height: number;
    public pointRadius = 3;
    public margin = 60;

    public chart: any;
    public svg: any;

    public xScale: any;
    public yScale: any;
    public xAxis: any;
    public yAxis: any;

    public logXScale = true;
    public logYScale = true;

    constructor(public element: HTMLElement, public xLabel = "X", public yLabel = "Y") {
        this.chart = d3.select(this.element);
        this.width = this.element.offsetWidth;
        this.height = Math.max(this.width / 2, 400);

        this.chart.selectAll("svg").data([0]).enter().append("svg");

        this.svg = this.chart.select("svg")
            .attr("width", this.width)
            .attr("height", this.height);

        this.xAxis = d3.axisBottom()
            .tickFormat((t: number) => t.toLocaleString())
            .ticks(4);

        this.yAxis = d3.axisLeft()
            .tickFormat((t: number) => t.toLocaleString())
            .ticks(5);
        this.resetScales();

        this.svg.append("g")
            .attr("class", "axis xaxis")
            .attr("transform", "translate(0," + (this.height - this.margin + 2 * this.pointRadius) + ")")
            .call(this.xAxis);

        this.svg.append("g")
            .attr("class", "axis yaxis")
            .attr("transform", "translate(" + (this.margin - 2 * this.pointRadius) + ",0)")
            .call(this.yAxis);

        this.svg.append("text")
            .attr("class", "ylabel")
            .attr("transform", "rotate(-90)")
            .attr("y", 0)
            .attr("x", 0 - (this.height / 2))
            .attr("dy", "1em")
            .style("font-weight", "600")
            .style("text-anchor", "middle")
            .text(this.yLabel);

        this.svg.append("text")
            .attr("class", "xlabel")
            .attr("transform", "translate(" + (this.width / 2) + " ," + (this.height - 20) + ")")
            .style("text-anchor", "middle")
            .style("font-weight", "600")
            .text(this.xLabel);

        window.addEventListener("resize", () => this.redraw());
        this.chart.select(".ylog").on("change", () => {
            this.setLogYScale(!this.logYScale);
        });

        this.chart.select(".xlog").on("change", () => {
            this.setLogXScale(!this.logXScale);
        });
    }

    public setLogXScale(l: boolean): void {
        this.logXScale = l;
        this.resetScales();
        this.redraw(true);
    }

    public setLogYScale(l: boolean): void {
        this.logYScale = l;
        this.resetScales();
        this.redraw(true);
    }

    public resetScales(): void {
        this.xScale = (this.logXScale ? d3.scaleLog() : d3.scaleLinear())
            .domain([1, 100000])
            .range([this.margin, this.width - this.margin]);

        this.yScale = (this.logYScale ? d3.scaleLog() : d3.scaleLinear())
            .domain([1, 100000])
            .range([this.margin, this.height - this.margin]);

        this.xAxis.scale(this.xScale);
        this.yAxis.scale(this.yScale);
    }

    public redraw(transition = false): void {
        this.width = this.element.offsetWidth;
        this.height = Math.max(this.width / 2, 400);

        this.svg.attr("width", this.width).attr("height", this.height);
        this.xScale.range([this.margin, this.width - this.margin]);
        this.yScale.range([this.margin, this.height - this.margin]);
        this.svg.select(".ylabel").attr("x", 0 - (this.height / 2));
        this.svg.select(".xlabel").attr("transform", "translate(" + (this.width / 2) + " ," + (this.height - 20) + ")");

        let selection = this.svg;
        if (transition) {
            selection = this.svg.transition().duration(1500);
        }
        selection.select(".xaxis")
            .attr("transform", "translate(0," + (this.height - this.margin + 2 * this.pointRadius) + ")")
            .call(this.xAxis);

        selection.select(".yaxis")
            .attr("transform", "translate(" + (this.margin - 2 * this.pointRadius) + ",0)")
            .call(this.yAxis);
    }
}
