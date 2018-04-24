import { countryEmoji } from "./country_emoji";
import { CountryTable } from "./country_table";
import { displayRank, ILocation } from "./location";

export class CityTable extends CountryTable {
    public filterCountry: string;

    public filterData(): ILocation[] {
        let current = this.data;
        if (this.filterCountry) {
            current = current.filter((l) => (l.country === this.filterCountry));
        }
        return current.slice(0, this.filterCount);
    }

    public formatLocation(l: ILocation): string {
        return (countryEmoji[l.country] || "") + " " + l.city + " "
            + (this.filterCountry ? displayRank(l.ranks[this.rank]) : "");
    }
}
