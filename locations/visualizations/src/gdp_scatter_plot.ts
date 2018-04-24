import { ILocation } from "./location";
import { ScatterPlot } from "./scatter_plot";

export class GDPScatterPlot extends ScatterPlot {
    constructor(element: HTMLElement, data: ILocation[]) {
        super(element, data, (x) => x.gdp, (x) => x.count,  "GDP");
    }

    public resetScales(): void {
        super.resetScales();
        this.xScale.domain([1000000000, 20000000000000]);
    }

    public xTickFormat(x: number): string {
        return "$" + (x / 1000000000).toLocaleString() + (this.element.offsetWidth > 500 ? " Billion" : "B");
    }
}
