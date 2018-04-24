import Slider from "slideyslider";
import { WorldMap } from "./worldmap";

declare var d3: any;
declare var topusers: any;

export interface IUser {
    login: string;
    name: string;
    location: string;
    company: string;
    country: string;
    lat: number;
    lng: number;
    followers: number;
    x?: number;
    y?: number;
}

function getUsers(count: number): IUser[] {
    const ret: IUser[] = [];
    for (let i = 0; i < count; ++i) {
        ret.push({
            company: topusers.company[i],
            country: topusers.country[i],
            followers: topusers.followers[i],
            lat: topusers.lat[i],
            lng: topusers.lng[i],
            location: topusers.location[i],
            login: topusers.login[i],
            name: topusers.name[i],
        });
    }
    return ret.reverse();
}

export class ForceDirectedDeveloperMap extends WorldMap {
    public count = 1024;
    public radius = 3;
    public collisionRadius = 3;

    public tooltip: any;
    public usersGroup: any;

    public simulation = d3.forceSimulation();
    public slider: Slider;

    constructor(element: HTMLElement) {
        super(element);

        this.adjustRadius();

        // Tooltip related
        this.tooltip = this.chart.append("div")
            .attr("class", "tooltip hidden");

        this.usersGroup = this.outer.append("g").attr("class", "users");

        // poor mans legend
        this.svg.append("circle")
            .attr("fill-opacity", .9)
            .attr("stroke", "white")
            .attr("stroke-width", this.strokeWidth)
            .attr("fill", "orange")
            .attr("r", this.radius)
            .attr("cx", this.radius * 4)
            .attr("cy", this.radius * 4);

        this.svg.append("text")
            .attr("x", this.radius * 6)
            .attr("y", this.radius * 5)
            .attr("font-size", "10px")
            .text("= 1 GitHub Account");

        const sliderDiv = document.createElement("div");
        sliderDiv.className = "slider";
        this.element.appendChild(sliderDiv);

        this.slider = new Slider(sliderDiv, "# of GitHub Accounts",
            (rate) => {
                rate = Math.floor(rate);
                this.slider.move(rate);
                this.displayUsers(rate);
            },
            {
                className: "col-xs-12 col-md-6 col-md-offset-3",
                domain: [16, 4096],
                format: (rate) => Math.floor(rate).toString(),
                initial: this.count,
                scale: d3.scaleLog().base(2),
                tickFormat: (rate) => rate.toString(),
                ticks: 4,
            });

        this.displayUsers(this.count);
    }

    public transitionMap(x: number, y: number, s: number, countryName: string = null): void {
        super.transitionMap(x, y, s, countryName);

        this.outer.transition("circles").duration(1500)
            .selectAll("circle")
            .attr("r", this.radius / this.scaling)
            .attr("stroke-width", this.strokeWidth / this.scaling);
        // this is a hack
        this.displayUsers(this.count);
    }

    public displayUsers(count: number): void {
        this.count = count;
        if (count >= 2000) {
            this.collisionRadius = this.radius / 3;
        } else if (count >= 1000) {
            this.collisionRadius = this.radius / 2;
        } else if (count >= 100) {
            this.collisionRadius = this.radius * 2 / 3;
        } else {
            this.collisionRadius = this.radius;
        }

        const users = getUsers(count);
        users.forEach((user: IUser) => {
            const coord = this.projection([user.lng, user.lat]);
            user.x = coord[0];
            user.y = coord[1];
            if (isNaN(user.x) || isNaN(user.y)) {
                user.x = user.y = -100;
            }
        });

        this.updateForceSimulation(users);
    }

    public updateForceSimulation(users: IUser[]): void {
        this.simulation.nodes(users).alpha(1.0).restart()
            .force("x", d3.forceX().x((d: any) => d.x))
            .force("y", d3.forceY().y((d: any) => d.y))
            .force("collision", d3.forceCollide().radius(
                Math.min(this.collisionRadius / Math.sqrt(this.scaling), this.radius / this.scaling)))
            .on("tick", () => {
                const group = this.usersGroup.selectAll("circle").data(users);
                group.enter()
                    .append("circle")
                    .attr("fill-opacity", .9)
                    .attr("stroke", "white")
                    .attr("fill", "orange")
                    .attr("stroke-width", this.strokeWidth / this.scaling)
                    .attr("r", this.radius / this.scaling)
                    .on("mousemove", (user: IUser) => {
                        const mouse = d3.mouse(this.svg.node()).map((d: number) => +d);

                        let extra = "";
                        if (user.company) {
                            extra += "<br>" + user.company;
                        }
                        const html = `<b>${(user.name || user.login)}</b>
                            <span style='color:rgb(102, 102, 102)'>${user.login}</span>
                            <br><i style='color:rgb(102, 102, 102)' class='fa fa-map-marker'>
                            </i> ${user.location + extra}
                            <br>${user.followers.toLocaleString()} followers`;

                        this.tooltip.classed("hidden", false)
                            .attr("style", `left:${mouse[0] + this.offsetLeft}px;top:${mouse[1] + this.offsetTop}px`)
                            .html(html);
                    })
                    .on("mouseout", () => this.tooltip.classed("hidden", true))
                    .on("click", (user: IUser) => {
                        if (this.previousZoom === user.country) {
                            window.open("https://github.com/" + user.login, "_blank");
                        } else {
                            this.zoomCountry(user.country);
                        }
                    })
                    .merge(group)
                    .attr("cx", (user: IUser) => user.x)
                    .attr("cy", (user: IUser) => user.y);

                group.exit().remove();
            });
    }

    public resize(): void {
        super.resize();
        this.adjustRadius();
        this.usersGroup.selectAll("circle")
            .attr("r", this.radius / this.scaling)
            .attr("stroke-width", this.strokeWidth / this.scaling);
        this.displayUsers(this.count);
    }

    protected adjustRadius(): void {
        if (this.width < 500) {
            this.radius = this.collisionRadius = 1.5;
            this.strokeWidth = 2 / 3;
        } else if (this.width < 800) {
            this.radius = this.collisionRadius = 2;
        } else {
            this.radius = this.collisionRadius = 3;
            this.strokeWidth = 1;
        }
    }
}
