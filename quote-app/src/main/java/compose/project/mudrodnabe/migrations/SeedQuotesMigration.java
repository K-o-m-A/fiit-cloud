package compose.project.mudrodnabe.migrations;

import compose.project.mudrodnabe.entities.Quote;
import io.mongock.api.annotations.ChangeUnit;
import io.mongock.api.annotations.Execution;
import io.mongock.api.annotations.RollbackExecution;
import lombok.extern.slf4j.Slf4j;
import org.springframework.data.mongodb.core.MongoTemplate;

import java.util.List;

@Slf4j
@ChangeUnit(id = "seed-quotes", order = "001", author = "dariushorvath")
public class SeedQuotesMigration {
    @Execution
    public void seedQuotes(MongoTemplate mongoTemplate) {
        log.info("Starting quote seed migration");
        List<Quote> quotes = List.of(
                new Quote(null, "The only way to do great work is to love what you do."),
                new Quote(null, "In the middle of every difficulty lies opportunity."),
                new Quote(null, "It does not matter how slowly you go as long as you do not stop.")
        );
        log.debug("Prepared {} quotes for insertion", quotes.size());

        mongoTemplate.insertAll(quotes);
        log.info("Seeded {} quotes into MongoDB", quotes.size());
    }

    @RollbackExecution
    public void rollback(MongoTemplate mongoTemplate) {
        log.warn("Rolling back quote seed migration — dropping quotes collection");
        mongoTemplate.dropCollection(Quote.class);
        log.info("Dropped quotes collection");
    }
}

