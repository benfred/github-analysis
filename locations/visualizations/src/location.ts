
export type Rank = "count" | "logfollowers" | "followers" | "accountsper1M" | "accountsper1Bgdp"
    | "population" | "gdp";

export let rankLabels: {[r in Rank]: string} = {
    accountsper1Bgdp: "Accounts / $1B GDP",
    accountsper1M: "Accounts / 1M Population",
    count: "GitHub Accounts",
    // followers: "∑ followers",
    followers: "Total Followers",
    gdp: "GDP",
    logfollowers: "∑ log(followers)",
    population: "Population",
};

export interface ILocation {
    country: string;
    city?: string;
    count: number;
    logfollowers: number;
    followers: number;
    accountsper1M: number;
    accountsper1Bgdp: number;
    population: number;
    gdp: number;
    ranks: {[r in Rank]?: number};
}

export function calculateRanks(data: ILocation[]): ILocation[] {
    data.forEach((d) => d.ranks = {});
    for (const r in rankLabels) {
        if (rankLabels.hasOwnProperty(r)) {
            const rank = r as Rank;
            data.sort((a, b) => b[rank] - a[rank]);
            let current = 0;
            let previous = Infinity;
            data.forEach((d) => {
                const val = d[rank];

                if (val !== previous) {
                    current++;
                }
                d.ranks[rank] = current;
                previous = val;
            });
        }
    }
    // resort to original-ish order
    data.sort((a, b) => b.count - a.count);
    return data;
}

// https://stackoverflow.com/questions/13627308/add-st-nd-rd-and-th-ordinal-suffix-to-a-number
function getOrdinal(n: number) {
    const s = ["th", "st", "nd", "rd"];
    const v = n % 100;
    return n + (s[(v - 20) % 10] || s[v] || s[0]);
}

export let displayRank = (n: number) => `<span style='color:rgb(102, 102, 102);'> (${getOrdinal(n)})</span>`;
