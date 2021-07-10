package com.skywire.skycoin.vpn.objects;

import com.skywire.skycoin.vpn.R;

public enum ServerRatings {
    Gold,
    Silver,
    Bronze;

    /**
     * Allows to get the resource ID of the string corresponding to the rating. If no resource is
     * found for the rating, -1 is returned.
     */
    public static int getTextForRating(ServerRatings rating) {
        if (rating == Gold) {
            return R.string.rating_gold;
        } else if (rating == Silver) {
            return R.string.rating_silver;
        } else if (rating == Bronze) {
            return R.string.rating_bronze;
        }

        return -1;
    }

    public static int getNumberForRating(ServerRatings rating) {
        if (rating == Gold) {
            return 2;
        } else if (rating == Silver) {
            return 1;
        } else if (rating == Bronze) {
            return 0;
        }

        return -1;
    }
}
