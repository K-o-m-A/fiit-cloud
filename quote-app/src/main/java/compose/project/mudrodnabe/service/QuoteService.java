package compose.project.mudrodnabe.service;

import compose.project.mudrodnabe.domain.QuoteDto;
import compose.project.mudrodnabe.entities.Quote;
import compose.project.mudrodnabe.repository.QuoteRepository;
import lombok.AllArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

import java.util.List;
import java.util.Random;

@Slf4j
@Service
@AllArgsConstructor
public class QuoteService {

    private final QuoteRepository quoteRepository;

    public QuoteDto getQuote() {
        log.debug("Querying repository for all quotes");
        List<Quote> quotes = quoteRepository.findAll();
        log.debug("Repository returned {} quote(s)", quotes.size());

        if (quotes.isEmpty()) {
            log.warn("Quote repository is empty — returning fallback response");
            return new QuoteDto("No quotes found");
        }

        // pick a random one
        Random random = new Random();
        int index = random.nextInt(quotes.size());
        log.trace("Selected random index {} from pool of {}", index, quotes.size());
        Quote randomQuote = quotes.get(index);

        log.info("Selected quote id={} out of {} total", randomQuote.getId(), quotes.size());
        return new QuoteDto(randomQuote.getQuote());
    }
}